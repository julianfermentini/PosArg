package arca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"testing"
)

// Certificados AFIP suelen venir en PKCS1 (openssl genrsa clásico), pero
// algunas herramientas emiten PKCS8 — parseRSAPrivateKey tiene que aceptar
// los dos formatos, si no una empresa con clave PKCS8 no puede autenticarse
// nunca contra WSAA.
func TestParseRSAPrivateKey_PKCS1YPKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("no se pudo generar la clave de prueba: %v", err)
	}

	pkcs1 := x509.MarshalPKCS1PrivateKey(key)
	if _, err := parseRSAPrivateKey(pkcs1); err != nil {
		t.Errorf("PKCS1: no esperaba error, dio: %v", err)
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("no se pudo generar PKCS8 de prueba: %v", err)
	}
	if _, err := parseRSAPrivateKey(pkcs8); err != nil {
		t.Errorf("PKCS8: no esperaba error, dio: %v", err)
	}
}

func TestParseRSAPrivateKey_Invalida(t *testing.T) {
	if _, err := parseRSAPrivateKey([]byte("esto no es una clave DER")); err == nil {
		t.Error("esperaba error con bytes inválidos, no dio ninguno")
	}
}
