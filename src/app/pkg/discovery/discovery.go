package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourorg/cluster-reflector/app/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// ClusterDiscovery manages discovery of cluster information
type ClusterDiscovery struct {
	clientset       kubernetes.Interface
	dynamicClient   dynamic.Interface
	runtimeClient   client.Client
	config          *types.Config
	logger          *logrus.Logger
	cache           *types.ClusterCache
	cacheMutex      sync.RWMutex
	stopCh          chan struct{}
}

// NewClusterDiscovery creates a new ClusterDiscovery instance
func NewClusterDiscovery(cfg *types.Config, logger *logrus.Logger) (*ClusterDiscovery, error) {
	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Get Kubernetes config
	restConfig, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create runtime client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	
	runtimeClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime client: %w", err)
	}

	return &ClusterDiscovery{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		runtimeClient: runtimeClient,
		config:        cfg,
		logger:        logger,
		cache: &types.ClusterCache{
			TTL: cfg.CacheTTL,
		},
		stopCh: make(chan struct{}),
	}, nil
}

// Start begins the discovery process
func (cd *ClusterDiscovery) Start(ctx context.Context) error {
	cd.logger.Info("Starting cluster discovery")
	
	// Log discovery configuration
	cd.logger.WithFields(logrus.Fields{
		"preferCRD":        cd.config.PreferCRD,
		"fallbackWorkloads": cd.config.FallbackWorkloads,
		"crdOnly":          cd.config.CRDOnly,
		"namespaceSelector": cd.config.NamespaceSelector,
		"workloadKinds":    cd.config.WorkloadKinds,
	}).Info("Discovery configuration")

	// Initial refresh
	if err := cd.refreshCache(ctx); err != nil {
		return fmt.Errorf("failed initial cache refresh: %w", err)
	}

	// Start periodic refresh
	ticker := time.NewTicker(cd.config.CacheTTL / 2) // Refresh at half the TTL
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cd.logger.Info("Stopping cluster discovery")
			return nil
		case <-cd.stopCh:
			cd.logger.Info("Discovery stopped")
			return nil
		case <-ticker.C:
			if err := cd.refreshCache(ctx); err != nil {
				cd.logger.WithError(err).Error("Failed to refresh cache")
			}
		}
	}
}

// Stop stops the discovery process
func (cd *ClusterDiscovery) Stop() {
	close(cd.stopCh)
}

// GetClusterInfo returns cached cluster information
func (cd *ClusterDiscovery) GetClusterInfo() *types.ClusterInfo {
	cd.cacheMutex.RLock()
	defer cd.cacheMutex.RUnlock()

	if cd.cache.Data == nil || cd.cache.IsExpired() {
		cd.logger.Warn("Cache is expired or empty")
		return &types.ClusterInfo{
			APIVersion: "reflector.grid.sce.com/v1",
			Timestamp:  time.Now(),
			Nodes:      []types.Node{},
			Apps:       []types.App{},
		}
	}

	// Update timestamp for current request
	info := *cd.cache.Data
	info.Timestamp = time.Now()
	return &info
}

// refreshCache updates the cache with current cluster information
func (cd *ClusterDiscovery) refreshCache(ctx context.Context) error {
	cd.logger.Debug("Refreshing cache")

	// Discover nodes
	nodes, err := cd.discoverNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover nodes: %w", err)
	}

	// Discover applications
	apps, err := cd.discoverApps(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover apps: %w", err)
	}

	// Update cache
	cd.cacheMutex.Lock()
	cd.cache.Data = &types.ClusterInfo{
		APIVersion: "reflector.grid.sce.com/v1",
		Timestamp:  time.Now(),
		Nodes:      nodes,
		Apps:       apps,
	}
	cd.cache.UpdatedAt = time.Now()
	cd.cacheMutex.Unlock()

	cd.logger.WithFields(logrus.Fields{
		"nodes": len(nodes),
		"apps":  len(apps),
	}).Debug("Cache refreshed")

	return nil
}

// discoverNodes discovers cluster nodes
func (cd *ClusterDiscovery) discoverNodes(ctx context.Context) ([]types.Node, error) {
	nodeList, err := cd.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodes := make([]types.Node, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		nodeInfo := types.Node{
			Name:    node.Name,
			IP:      cd.getNodeInternalIP(&node),
			Role:    cd.getNodeRole(&node),
			Version: node.Status.NodeInfo.KubeletVersion,
		}
		nodes = append(nodes, nodeInfo)
	}

	return nodes, nil
}

// getNodeInternalIP extracts the internal IP of a node
func (cd *ClusterDiscovery) getNodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

// getNodeRole determines the role of a node
func (cd *ClusterDiscovery) getNodeRole(node *corev1.Node) string {
	// Check for control-plane labels
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return "control-plane"
	}
	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return "control-plane"
	}

	// Check for control-plane taints
	for _, taint := range node.Spec.Taints {
		if strings.Contains(taint.Key, "control-plane") || strings.Contains(taint.Key, "master") {
			if taint.Effect == corev1.TaintEffectNoSchedule {
				return "control-plane"
			}
		}
	}

	return "worker"
}

// discoverApps discovers applications in the cluster
func (cd *ClusterDiscovery) discoverApps(ctx context.Context) ([]types.App, error) {
	appMap := make(map[string]*types.App)

	// Try CRD discovery first if enabled
	if cd.config.PreferCRD {
		if err := cd.discoverAppsFromCRD(ctx, appMap); err != nil {
			cd.logger.WithError(err).Warn("CRD discovery failed, falling back to workloads")
		}
	}

	// Fallback to workload discovery if enabled and not CRD-only mode
	if cd.config.FallbackWorkloads && !cd.config.CRDOnly {
		if err := cd.discoverAppsFromWorkloads(ctx, appMap); err != nil {
			cd.logger.WithError(err).Error("Workload discovery failed")
		}
	} else if cd.config.CRDOnly {
		cd.logger.Debug("CRD-only mode enabled, skipping workload discovery")
	}

	// Convert map to slice
	apps := make([]types.App, 0, len(appMap))
	for _, app := range appMap {
		apps = append(apps, *app)
	}

	return apps, nil
}

// discoverAppsFromCRD discovers apps from AppVersion CRDs
func (cd *ClusterDiscovery) discoverAppsFromCRD(ctx context.Context, appMap map[string]*types.App) error {
	// Define AppVersion GVR
	gvr := schema.GroupVersionResource{
		Group:    "cluster.grid.sce.com",
		Version:  "v1alpha1",
		Resource: "appversions",
	}

	// List AppVersions
	if cd.config.NamespaceSelector == "" {
		// List from all namespaces
		list, err := cd.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list AppVersions: %w", err)
		}
		// Process the list items and add to appMap
		for _, item := range list.Items {
			cd.processAppVersionFromUnstructured(item.Object, appMap)
		}
	} else {
		// Parse namespace selector and list from specific namespaces
		namespaces := cd.parseNamespaceSelector(cd.config.NamespaceSelector)
		for _, ns := range namespaces {
			list, err := cd.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				cd.logger.WithError(err).WithField("namespace", ns).Warn("Failed to list AppVersions in namespace")
				continue
			}
			// Process the list items and add to appMap
			for _, item := range list.Items {
				cd.processAppVersionFromUnstructured(item.Object, appMap)
			}
		}
	}

	return nil
}

// discoverAppsFromWorkloads discovers apps from workload metadata
func (cd *ClusterDiscovery) discoverAppsFromWorkloads(ctx context.Context, appMap map[string]*types.App) error {
	namespaces := []string{""}
	if cd.config.NamespaceSelector != "" {
		namespaces = cd.parseNamespaceSelector(cd.config.NamespaceSelector)
	}

	for _, kind := range cd.config.WorkloadKinds {
		switch kind {
		case "Deployment":
			if err := cd.discoverFromDeployments(ctx, namespaces, appMap); err != nil {
				cd.logger.WithError(err).Error("Failed to discover from deployments")
			}
		case "StatefulSet":
			if err := cd.discoverFromStatefulSets(ctx, namespaces, appMap); err != nil {
				cd.logger.WithError(err).Error("Failed to discover from statefulsets")
			}
		}
	}

	return nil
}

// discoverFromDeployments discovers apps from deployments
func (cd *ClusterDiscovery) discoverFromDeployments(ctx context.Context, namespaces []string, appMap map[string]*types.App) error {
	for _, ns := range namespaces {
		var deployments *appsv1.DeploymentList
		var err error

		if ns == "" {
			// List from all namespaces
			deployments, err = cd.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		} else {
			// List from specific namespace
			deployments, err = cd.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		}

		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		for _, deployment := range deployments.Items {
			cd.processWorkloadLabels(deployment.Labels, deployment.Spec.Template.Spec.Containers, appMap)
		}
	}

	return nil
}

// discoverFromStatefulSets discovers apps from statefulsets
func (cd *ClusterDiscovery) discoverFromStatefulSets(ctx context.Context, namespaces []string, appMap map[string]*types.App) error {
	for _, ns := range namespaces {
		var statefulSets *appsv1.StatefulSetList
		var err error

		if ns == "" {
			statefulSets, err = cd.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		} else {
			statefulSets, err = cd.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		}

		if err != nil {
			return fmt.Errorf("failed to list statefulsets: %w", err)
		}

		for _, sts := range statefulSets.Items {
			cd.processWorkloadLabels(sts.Labels, sts.Spec.Template.Spec.Containers, appMap)
		}
	}

	return nil
}

// processWorkloadLabels processes workload labels to extract app information
func (cd *ClusterDiscovery) processWorkloadLabels(labels map[string]string, containers []corev1.Container, appMap map[string]*types.App) {
	appName := labels["app.kubernetes.io/name"]
	appVersion := labels["app.kubernetes.io/version"]

	// If no labels, try to parse from first container image
	if appName == "" && len(containers) > 0 {
		appName, appVersion = cd.parseImageTag(containers[0].Image)
	}

	if appName != "" {
		if appVersion == "" {
			appVersion = "unknown"
		}

		if existing, exists := appMap[appName]; exists {
			// Add version to variants if not already present
			found := false
			for _, variant := range existing.Variants {
				if variant == appVersion {
					found = true
					break
				}
			}
			if !found {
				existing.Variants = append(existing.Variants, appVersion)
			}
		} else {
			appMap[appName] = &types.App{
				Name:     appName,
				Version:  appVersion,
				Variants: []string{appVersion},
			}
		}
	}
}

// parseImageTag extracts app name and version from container image tag
func (cd *ClusterDiscovery) parseImageTag(image string) (string, string) {
	// Remove registry prefix if present
	parts := strings.Split(image, "/")
	imageName := parts[len(parts)-1]

	// Split name and tag
	nameTag := strings.Split(imageName, ":")
	if len(nameTag) < 2 {
		return nameTag[0], "latest"
	}

	name := nameTag[0]
	tag := nameTag[1]

	// Remove @sha256: suffix if present
	if strings.Contains(tag, "@") {
		tag = strings.Split(tag, "@")[0]
	}

	return name, tag
}

// processAppVersionFromUnstructured processes an AppVersion from unstructured data
func (cd *ClusterDiscovery) processAppVersionFromUnstructured(obj map[string]interface{}, appMap map[string]*types.App) {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return
	}

	name, ok := spec["name"].(string)
	if !ok {
		return
	}

	version, ok := spec["version"].(string)
	if !ok {
		return
	}

	if existing, exists := appMap[name]; exists {
		// Add version to variants if not already present
		found := false
		for _, variant := range existing.Variants {
			if variant == version {
				found = true
				break
			}
		}
		if !found {
			existing.Variants = append(existing.Variants, version)
		}
		// Update main version to latest
		existing.Version = version
	} else {
		appMap[name] = &types.App{
			Name:     name,
			Version:  version,
			Variants: []string{version},
		}
	}
}

// parseNamespaceSelector parses namespace selector string
func (cd *ClusterDiscovery) parseNamespaceSelector(selector string) []string {
	if selector == "" {
		return []string{""}
	}

	// Simple comma-separated namespace list for now
	// TODO: Implement label selector parsing
	namespaces := strings.Split(selector, ",")
	for i, ns := range namespaces {
		namespaces[i] = strings.TrimSpace(ns)
	}

	return namespaces
}

// HealthCheck performs a basic health check
func (cd *ClusterDiscovery) HealthCheck(ctx context.Context) error {
	// Try to list nodes as a basic connectivity check
	_, err := cd.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	// Check if cache is reasonably fresh
	cd.cacheMutex.RLock()
	cacheAge := time.Since(cd.cache.UpdatedAt)
	cd.cacheMutex.RUnlock()

	if cacheAge > cd.config.CacheTTL*2 {
		return fmt.Errorf("cache is stale (age: %s)", cacheAge)
	}

	return nil
}

// validateConfig validates the discovery configuration
func validateConfig(cfg *types.Config) error {
	if cfg.CRDOnly && !cfg.PreferCRD {
		return fmt.Errorf("CRD-only mode requires preferCRD to be true")
	}
	
	if cfg.CRDOnly && cfg.FallbackWorkloads {
		logrus.Warn("CRD-only mode enabled but fallbackWorkloads is true - workloads will be ignored")
	}
	
	return nil
}
