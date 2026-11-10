// Package links parses proxy share links (vless://, vmess://, trojan://, ss://,
// hysteria2://, tuic://) into a protocol-agnostic ServerSpec that the engine
// layer can turn into an outbound configuration.
package links

// Transport describes the stream transport used to reach a server.
type Transport struct {
	// Kind is one of: tcp, ws, grpc, http, httpupgrade, quic, kcp.
	Kind string
	// Path is the HTTP/WS request path (ws, http, httpupgrade).
	Path string
	// Host is the HTTP Host header override (ws, http, httpupgrade).
	Host string
	// ServiceName is the gRPC service name.
	ServiceName string
	// Mode is the gRPC mode or the QUIC mode (e.g. "gun", "multi").
	Mode string
	// HeaderType is the gRPC header type (e.g. "gun", "srtp").
	HeaderType string
}

// RealityOptions holds the REALITY handshake parameters.
type RealityOptions struct {
	PublicKey string
	ShortID   string
	SpiderX   string
}

// TLSOptions describes the TLS layer of an outbound.
type TLSOptions struct {
	Enabled bool
	// ServerName is the SNI presented to the server.
	ServerName string
	// ALPN is the negotiated application protocol list.
	ALPN []string
	// Insecure disables certificate verification.
	Insecure bool
	// UTLSFingerprint is the uTLS fingerprint to mimic (e.g. "chrome").
	UTLSFingerprint string
	// Reality is non-nil when the REALITY transport is used.
	Reality *RealityOptions
}

// ServerSpec is a protocol-agnostic description of a single proxy server.
type ServerSpec struct {
	// Protocol is one of: vless, vmess, trojan, shadowsocks, hysteria2, tuic.
	Protocol string
	// Tag is the human readable server name.
	Tag string
	// Address is the server host or IP.
	Address string
	// Port is the server port.
	Port int

	// UUID is the user identifier (vless, vmess, tuic).
	UUID string
	// Flow is the XTLS flow control mode (vless).
	Flow string
	// Password is the credential (trojan, shadowsocks, hysteria2, tuic).
	Password string
	// Method is the cipher (shadowsocks).
	Method string

	Transport Transport
	TLS       TLSOptions

	// Obfs is the obfuscation mode (hysteria2, e.g. "salamander").
	Obfs string
	// ObfsPassword is the obfuscation password (hysteria2).
	ObfsPassword string
	// CongestionControl is the congestion controller (tuic, e.g. "bbr").
	CongestionControl string
	// UDPRelayMode is the UDP relay strategy (tuic, e.g. "native").
	UDPRelayMode string
	// UpMbps / DownMbps are the optional bandwidth hints (hysteria2).
	UpMbps   int
	DownMbps int
}

// DisplayName returns a human readable label for the server.
func (s *ServerSpec) DisplayName() string {
	if s.Tag != "" {
		return s.Tag
	}
	return s.Protocol + " " + s.Address
}
