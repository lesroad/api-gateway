package signature

import (
	"fmt"
)

type SignatureFactory struct{}

func NewSignatureFactory() *SignatureFactory {
	return &SignatureFactory{}
}

func (f *SignatureFactory) CreateGenerator(signatureType string, config map[string]string) (SignatureGenerator, error) {
	switch signatureType {
	case TypeXKW:
		appId, ok := config["app_id"]
		if !ok {
			return nil, fmt.Errorf("missing app_key for XKW signature")
		}
		appSecret, ok := config["app_secret"]
		if !ok {
			return nil, fmt.Errorf("missing app_secret for XKW signature")
		}
		return NewXKWSignatureGenerator(appId, appSecret), nil

	case TypeHMAC:
		accessKey, ok := config["access_key"]
		if !ok {
			return nil, fmt.Errorf("missing access_key for HMAC signature")
		}
		secretKey, ok := config["secret_key"]
		if !ok {
			return nil, fmt.Errorf("missing secret_key for HMAC signature")
		}
		return NewHMACSignatureGenerator(accessKey, secretKey), nil

	default:
		return nil, fmt.Errorf("unsupported signature type: %s", signatureType)
	}
}
