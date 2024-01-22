package protocol

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var _ Protocol = (*AesTransport)(nil)

type AesTransport struct {
	config       *DeviceConfig
	loginVersion int

	key     *rsa.PrivateKey
	session *AesEncryptedSession

	handshakeDone bool
	sessionExpiry time.Time
	cookies       map[string]string
	loginToken    string

	httpClient *http.Client

	commonHeaders map[string]string

	logger log.Logger
}

type AesProtoBaseRequest struct {
	Method string `json:"method"`
}

type AesProtoHandshakeRequestParams struct {
	Key string `json:"key"`
}

type AesProtoHandshakeRequest struct {
	AesProtoBaseRequest
	Params AesProtoHandshakeRequestParams `json:"params"`
}

type AesHandshakeResponseResult struct {
	Key string `json:"key"`
}

type AesHandshakeResponse struct {
	AesProtoBaseResponse
	Result AesHandshakeResponseResult `json:"result"`
}

type AesProtoBaseResponse struct {
	ErrorCode int `json:"error_code"`
}

type AesLoginRequest struct {
	AesProtoBaseRequest
	Params            map[string]string `json:"params"`
	RequestTimeMillis int64             `json:"request_time_milis"`
}

type AesLoginResponseResult struct {
	Token string `json:"token"`
}

type AesLoginResponse struct {
	AesProtoBaseResponse
	Result AesLoginResponseResult `json:"result"`
}

type AesPassthroughRequestParams struct {
	Request string `json:"request"`
}

type AesPassthroughRequest struct {
	AesProtoBaseRequest
	Params AesPassthroughRequestParams `json:"params"`
}

type AesPassthroughResponseResult struct {
	Response string `json:"response"`
}

type AesPassthroughResponse struct {
	AesProtoBaseResponse
	Result AesPassthroughResponseResult `json:"result"`
}

func NewAesTransport(key *rsa.PrivateKey, config *DeviceConfig, logger log.Logger) (*AesTransport, error) {
	return &AesTransport{
		key:    key,
		config: config,

		loginVersion:  2,
		handshakeDone: false,

		httpClient: &http.Client{},
		commonHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"requestByApp": "true",
		},
		cookies:       make(map[string]string),
		sessionExpiry: time.Now(),

		logger: logger,
	}, nil
}

func (t *AesTransport) Send(request, response interface{}) error {
	if !t.handshakeDone || t.handshakeExpired() {
		err := t.handshake()

		if err != nil {
			return err
		}
	}

	if t.loginToken == "" {
		err := t.login()

		if err != nil {
			return err
		}
	}

	err := t.securePassthrough(request, response)

	if err != nil {
		// assume session expired
		// TODO: handle error codes
		t.loginToken = ""
		t.session = nil
		t.sessionExpiry = time.Now()
		t.handshakeDone = false
		return err
	}

	return nil
}

func (t *AesTransport) Close() error {
	return nil
}

func (t *AesTransport) handshake() error {

	level.Debug(t.logger).Log("msg", "performing handshake", "target", t.config.Address)

	encoded, err := x509.MarshalPKIXPublicKey(&t.key.PublicKey)

	if err != nil {
		return err
	}

	pubKeyPem := new(bytes.Buffer)
	pem.Encode(pubKeyPem, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: encoded,
	})

	request := &AesProtoHandshakeRequest{
		AesProtoBaseRequest: AesProtoBaseRequest{
			Method: "handshake",
		},
		Params: AesProtoHandshakeRequestParams{
			Key: pubKeyPem.String(),
		},
	}

	marshalled, err := json.Marshal(request)

	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/app", t.config.Address), bytes.NewBuffer(marshalled))

	if err != nil {
		return err
	}

	// keys need to be case sensitive
	for k, v := range t.commonHeaders {
		req.Header[k] = []string{v}
	}

	res, err := t.httpClient.Do(req)

	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("handshake failed with status code %d", res.StatusCode)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var response AesHandshakeResponse

	err = json.Unmarshal(body, &response)

	if err != nil {
		return err
	}

	if response.ErrorCode != 0 {
		return fmt.Errorf("handshake failed with error code %d", response.ErrorCode)
	}

	for _, c := range res.Cookies() {
		t.cookies[c.Name] = c.Value
	}

	t.sessionExpiry = time.Now().Add(time.Duration(86400) * time.Second)
	t.session, err = NewAesEncryptedSession(response.Result.Key, t.key)

	if err != nil {
		return err
	}

	t.handshakeDone = true

	return nil
}

func (t *AesTransport) handshakeExpired() bool {
	return time.Now().After(t.sessionExpiry)
}

func (t *AesTransport) login() error {

	level.Debug(t.logger).Log("msg", "performing login", "target", t.config.Address)

	req := &AesLoginRequest{
		AesProtoBaseRequest: AesProtoBaseRequest{
			Method: "login_device",
		},
		Params:            t.loginParameters(),
		RequestTimeMillis: time.Now().UnixMilli(),
	}

	var res AesLoginResponse

	err := t.securePassthrough(req, &res)

	if err != nil {
		return err
	}

	if res.ErrorCode != 0 {
		return fmt.Errorf("login failed with error code %d", res.ErrorCode)
	}

	t.loginToken = res.Result.Token

	return nil
}

func (t *AesTransport) securePassthrough(request interface{}, response interface{}) error {

	url := fmt.Sprintf("http://%s/app", t.config.Address)
	if t.loginToken != "" {
		url = fmt.Sprintf("%s?token=%s", url, t.loginToken)
	}

	marshalledRequest, err := json.Marshal(request)

	if err != nil {
		return err
	}

	level.Debug(t.logger).Log("msg", "sending request", "request", string(marshalledRequest))

	encrypted, err := t.session.Encrypt(marshalledRequest)

	if err != nil {
		return err
	}

	marshalled, err := json.Marshal(&AesPassthroughRequest{
		AesProtoBaseRequest: AesProtoBaseRequest{
			Method: "securePassthrough",
		},
		Params: AesPassthroughRequestParams{
			Request: base64.StdEncoding.EncodeToString(encrypted),
		},
	})

	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(marshalled))

	if err != nil {
		return err
	}

	for k, v := range t.commonHeaders {
		req.Header[k] = []string{v}
	}

	for k, v := range t.cookies {
		req.AddCookie(&http.Cookie{
			Name:  k,
			Value: v,
		})
	}

	httpRes, err := t.httpClient.Do(req)

	if err != nil {
		return err
	}

	defer httpRes.Body.Close()

	body, err := io.ReadAll(httpRes.Body)

	if err != nil {
		return err
	}

	var res AesPassthroughResponse

	err = json.Unmarshal(body, &res)

	if err != nil {
		return err
	}

	level.Debug(t.logger).Log("msg", "received encrypted response", "encrypted", res.Result.Response)

	if res.ErrorCode != 0 {
		return fmt.Errorf("passthrough failed with error code %d", res.ErrorCode)
	}

	decoded, err := base64.StdEncoding.DecodeString(res.Result.Response)

	if err != nil {
		return err
	}

	decrypted, err := t.session.Decrypt(decoded)

	if err != nil {
		return err
	}

	level.Debug(t.logger).Log("msg", "decrypted response", "response", string(decrypted))

	return json.Unmarshal(decrypted, response)
}

func (t *AesTransport) loginParameters() map[string]string {
	params := make(map[string]string)

	user, pass := t.hashCredentials(t.loginVersion == 2)

	params["username"] = user

	if t.loginVersion == 2 {
		params["password2"] = pass
	} else {
		params["password"] = pass
	}

	return params
}

func (t *AesTransport) hashCredentials(v2 bool) (string, string) {
	user := base64.StdEncoding.EncodeToString([]byte(sha1Hash([]byte(t.config.Credentials.Username))))
	var pass string

	if v2 {
		pass = base64.StdEncoding.EncodeToString([]byte(sha1Hash([]byte(t.config.Credentials.Password))))
	} else {
		pass = base64.StdEncoding.EncodeToString([]byte(t.config.Credentials.Password))
	}

	return user, pass
}

type AesEncryptedSession struct {
	block cipher.Block

	iv []byte
}

func NewAesEncryptedSession(handshakeKey string, key *rsa.PrivateKey) (*AesEncryptedSession, error) {
	handshakeData, err := base64.StdEncoding.DecodeString(handshakeKey)

	if err != nil {
		return nil, err
	}

	keyAndIv, err := key.Decrypt(nil, handshakeData, nil)

	if err != nil {
		return nil, err
	}

	sessionKey := keyAndIv[:16]
	sessionIv := keyAndIv[16:]

	block, err := aes.NewCipher(sessionKey)

	if err != nil {
		return nil, err
	}

	return &AesEncryptedSession{
		block: block,
		iv:    sessionIv,
	}, nil
}

func (s *AesEncryptedSession) Encrypt(data []byte) ([]byte, error) {
	padded, err := pkcs7Pad(data, s.block.BlockSize())

	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(padded))

	encryptor := cipher.NewCBCEncrypter(s.block, s.iv)

	encryptor.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

func (s *AesEncryptedSession) Decrypt(data []byte) ([]byte, error) {
	plaintext := make([]byte, len(data))

	decryptor := cipher.NewCBCDecrypter(s.block, s.iv)

	decryptor.CryptBlocks(plaintext, data)

	return pkcs7Unpad(plaintext, s.block.BlockSize())
}
