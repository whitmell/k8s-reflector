package types

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterInfo represents the main response structure
type ClusterInfo struct {
	APIVersion string    `json:"apiVersion"`
	Timestamp  time.Time `json:"timestamp"`
	Nodes      []Node    `json:"nodes"`
	Apps       []App     `json:"apps"`
}

// Node represents a cluster node
type Node struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Role    string `json:"role"`
	Version string `json:"version"`
}

// App represents an application with version information
type App struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Variants []string `json:"variants"`
}

// AppVersion is our custom CRD structure
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type AppVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppVersionSpec   `json:"spec,omitempty"`
	Status AppVersionStatus `json:"status,omitempty"`
}

// AppVersionSpec defines the desired state of AppVersion
type AppVersionSpec struct {
	// Name is the name of the application
	Name string `json:"name"`
	// Version is the version of the application
	Version string `json:"version"`
}

// AppVersionStatus defines the observed state of AppVersion
type AppVersionStatus struct {
	// ObservedAt is the timestamp when this version was last observed
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
}

// AppVersionList contains a list of AppVersion
// +kubebuilder:object:root=true
type AppVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppVersion `json:"items"`
}

// Config holds the application configuration
type Config struct {
	Listen              string
	CacheTTL            time.Duration
	NamespaceSelector   string
	PreferCRD           bool
	FallbackWorkloads   bool
	LogLevel            string
	WorkloadKinds       []string
	MetricsEnabled      bool
	HealthcheckMode     bool
}

// ClusterCache holds cached cluster information
type ClusterCache struct {
	Data      *ClusterInfo
	UpdatedAt time.Time
	TTL       time.Duration
}

// IsExpired checks if the cache is expired
func (c *ClusterCache) IsExpired() bool {
	return time.Since(c.UpdatedAt) > c.TTL
}
