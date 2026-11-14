//go:build integration

package messaging_test

import (
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/messaging/messagingtest"
	"github.com/testcontainers/testcontainers-go"
)

const rabbitMQPassword = messagingtest.Password

func startRabbitMQ(t *testing.T) (testcontainers.Container, string) {
	t.Helper()
	return messagingtest.Start(t)
}

func rabbitMQURL(address string, password string) string {
	return messagingtest.URL(address, password)
}

func TestConnectIntegration(t *testing.T) {
	_, address := startRabbitMQ(t)

	t.Run("connects with valid credentials", func(t *testing.T) {
		conn, err := messaging.Connect(rabbitMQURL(address, rabbitMQPassword), "shortbit-test")
		if err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })

		if conn.IsClosed() {
			t.Fatal("Connect() returned a closed connection")
		}
	})

	t.Run("rejects invalid credentials without leaking them", func(t *testing.T) {
		wrongPassword := "wrong-secret-password"

		conn, err := messaging.Connect(rabbitMQURL(address, wrongPassword), "shortbit-test")
		if err == nil {
			_ = conn.Close()
			t.Fatal("Connect() error = nil, want authentication error")
		}

		if strings.Contains(err.Error(), wrongPassword) {
			t.Errorf("Connect() error leaks the password: %q", err)
		}
	})
}
