package api

type Version struct {
	Meta    bool   `json:"meta"`
	Version string `json:"version"`
}

type Configs struct {
	Mode     string `json:"mode"`
	LogLevel string `json:"log-level"`
}

type Proxy struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Now   string   `json:"now"`
	All   []string `json:"all"`
	Alive bool     `json:"alive"`
}

type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type Traffic struct {
	Up        int64 `json:"up"`
	Down      int64 `json:"down"`
	UpTotal   int64 `json:"upTotal"`
	DownTotal int64 `json:"downTotal"`
}

type SubscriptionInfo struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
	Expire   int64 `json:"expire"`
}

type ProxyProvider struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	VehicleType      string            `json:"vehicleType"`
	Proxies          []Proxy           `json:"proxies"`
	UpdatedAt        string            `json:"updatedAt"`
	SubscriptionInfo *SubscriptionInfo `json:"subscriptionInfo"`
}

type ProvidersResponse struct {
	Providers map[string]ProxyProvider `json:"providers"`
}
