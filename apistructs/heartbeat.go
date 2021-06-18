package apistructs

// HeartBeatReq heatbeat request struct between probe-master and probe-agent
type HeartBeatReq struct {
	Name           string `json:"name"`
	SecretKey      string `json:"secretKey"`
	Address        string `json:"address"`
	CaData         string `json:"caData"`
	CertData       string `json:"certData"`
	KeyData        string `json:"keyData"`
	Token          string `json:"token"`
	Version        string `json:"version"`
	NodeCount      int    `json:"nodeCount"`
	ProbeNamespace string `json:"probeNamespace"`
}
