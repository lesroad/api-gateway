package signature

type SignatureGenerator interface {
	GenerateHeaders(method, path string, body []byte, params map[string]string) (map[string]string, error)
	GetType() string
}

const (
	TypeXKW    = "xkw"
	TypeHMAC   = "hmac"
)
