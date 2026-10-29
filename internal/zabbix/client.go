// Package zabbix fala com a API JSON-RPC do Zabbix 6.4.
package zabbix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 120 * time.Second

// Client é uma sessão autenticada. O token vive só na memória do processo.
type Client struct {
	url    string
	token  string
	http   *http.Client
	nextID int
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      int    `json:"id"`
	Auth    string `json:"auth,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (e *rpcError) Error() string {
	msg := strings.TrimSpace(e.Message)
	data := strings.TrimSpace(e.Data)
	if data != "" && !strings.EqualFold(data, msg) {
		return fmt.Sprintf("erro do Zabbix: %s (%s)", msg, data)
	}
	if msg == "" {
		return "erro do Zabbix"
	}
	return "erro do Zabbix: " + msg
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// Login autentica com usuário e senha e devolve um cliente pronto.
func Login(ctx context.Context, apiURL, username, password string) (*Client, error) {
	c := &Client{
		url:  apiURL,
		http: &http.Client{Timeout: requestTimeout},
	}
	var token string
	err := c.call(ctx, "user.login", map[string]string{
		"username": username,
		"password": password,
	}, &token)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "password") || strings.Contains(strings.ToLower(err.Error()), "login") {
			return nil, fmt.Errorf("usuário ou senha rejeitados pelo Zabbix")
		}
		return nil, fmt.Errorf("falha ao autenticar no Zabbix: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("o Zabbix não devolveu um token de sessão")
	}
	c.token = token
	return c, nil
}

// Version devolve a versão da API. Falha aqui não impede o relatório.
func (c *Client) Version(ctx context.Context) (string, error) {
	var version string
	if err := c.call(ctx, "apiinfo.version", map[string]any{}, &version); err != nil {
		return "", err
	}
	return version, nil
}

func (c *Client) call(ctx context.Context, method string, params any, dest any) error {
	if params == nil {
		params = map[string]any{}
	}
	c.nextID++
	payload := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID,
		Auth:    c.token,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("não foi possível montar a chamada %s: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("não foi possível montar a chamada %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")
	req.Header.Set("User-Agent", "zbx-admin")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao conectar em %s: %w", c.url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("falha ao ler a resposta de %s: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("o Zabbix respondeu HTTP %d em %s", resp.StatusCode, method)
	}
	var decoded rpcResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("resposta inválida de %s", method)
	}
	if decoded.Error != nil {
		return decoded.Error
	}
	if dest == nil {
		return nil
	}
	if len(decoded.Result) == 0 || string(decoded.Result) == "null" || string(decoded.Result) == "false" {
		return nil
	}
	if err := json.Unmarshal(decoded.Result, dest); err != nil {
		return fmt.Errorf("resposta inesperada de %s: %w", method, err)
	}
	return nil
}
