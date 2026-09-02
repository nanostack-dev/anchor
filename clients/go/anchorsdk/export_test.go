package anchorsdk

import "time"

func SignWebhookForTest(secret, msgID string, timestamp time.Time, body []byte) (string, error) {
	key, err := decodeSigningSecret(secret)
	if err != nil {
		return "", err
	}
	return signWebhook(key, msgID, timestamp, body), nil
}
