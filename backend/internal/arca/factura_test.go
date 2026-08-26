package arca

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Respuesta real de WSFE ante un comprobante autorizado, recortada a lo que
// parseCAEResponse mira. Formato tomado de la doc de FECAESolicitar.
const respuestaAutorizada = `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <FECAESolicitarResponse xmlns="http://ar.gov.afip.dif.FEV1/">
      <FECAESolicitarResult>
        <FeCabResp>
          <Resultado>A</Resultado>
        </FeCabResp>
        <FeDetResp>
          <FECAEDetResponse>
            <Concepto>1</Concepto>
            <DocTipo>99</DocTipo>
            <DocNro>0</DocNro>
            <CbteDesde>42</CbteDesde>
            <CbteHasta>42</CbteHasta>
            <CAE>71234567890123</CAE>
            <CAEFchVto>20250215</CAEFchVto>
            <Resultado>A</Resultado>
          </FECAEDetResponse>
        </FeDetResp>
      </FECAESolicitarResult>
    </FECAESolicitarResponse>
  </soapenv:Body>
</soapenv:Envelope>`

// Rechazo con una Observación (ej. importe mal calculado) — sin nodo Errors.
const respuestaRechazadaConObservacion = `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <FECAESolicitarResponse xmlns="http://ar.gov.afip.dif.FEV1/">
      <FECAESolicitarResult>
        <FeCabResp>
          <Resultado>R</Resultado>
        </FeCabResp>
        <FeDetResp>
          <FECAEDetResponse>
            <Resultado>R</Resultado>
            <Observaciones>
              <Obs>
                <Code>10016</Code>
                <Msg>Cbte Nro no cumple con formato correcto</Msg>
              </Obs>
            </Observaciones>
          </FECAEDetResponse>
        </FeDetResp>
      </FECAESolicitarResult>
    </FECAESolicitarResponse>
  </soapenv:Body>
</soapenv:Envelope>`

// SOAP Fault (ej. token vencido) — no llega a tener FeDetResp.
const respuestaSoapFault = `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <soapenv:Fault>
      <faultcode>ns1:HttpError</faultcode>
      <faultstring>Token o sign inválido</faultstring>
    </soapenv:Fault>
  </soapenv:Body>
</soapenv:Envelope>`

func TestParseCAEResponse_Autorizado(t *testing.T) {
	cae, fchVto, err := parseCAEResponse([]byte(respuestaAutorizada))
	if err != nil {
		t.Fatalf("no esperaba error, dio: %v", err)
	}
	if cae != "71234567890123" {
		t.Errorf("CAE = %q, esperaba %q", cae, "71234567890123")
	}
	if fchVto != "20250215" {
		t.Errorf("CAEFchVto = %q, esperaba %q", fchVto, "20250215")
	}
}

func TestParseCAEResponse_RechazadoIncluyeObservacion(t *testing.T) {
	_, _, err := parseCAEResponse([]byte(respuestaRechazadaConObservacion))
	if err == nil {
		t.Fatal("esperaba error por rechazo, no dio ninguno")
	}
	if !strings.Contains(err.Error(), "10016") || !strings.Contains(err.Error(), "formato correcto") {
		t.Errorf("el error debería incluir código y mensaje de la observación, dio: %v", err)
	}
}

func TestParseCAEResponse_SoapFaultDaErrorLegible(t *testing.T) {
	// Un SOAP Fault no tiene FeDetResp: Resultado queda como cadena vacía, que
	// parseCAEResponse trata igual que un rechazo (!= "A"), sin panic.
	_, _, err := parseCAEResponse([]byte(respuestaSoapFault))
	if err == nil {
		t.Fatal("esperaba error, no dio ninguno")
	}
}

func TestParseCAEResponse_XMLInvalido(t *testing.T) {
	_, _, err := parseCAEResponse([]byte("no es xml"))
	if err == nil {
		t.Fatal("esperaba error de parseo, no dio ninguno")
	}
}

func TestBuildQR_CamposYFormatoOficial(t *testing.T) {
	params := SolicitarCAEParams{
		CUIT:                   20123456789,
		PuntoVenta:             1,
		TipoCmp:                TipoFacturaB,
		Total:                  121.0,
		DocTipoRec:             TipoDocConsumidorFinal,
		DocNroRec:              0,
		CondicionIVAReceptorId: 5,
	}
	fecha, err := time.Parse("2006-01-02", "2025-01-15")
	if err != nil {
		t.Fatalf("fecha de test inválida: %v", err)
	}
	params.Fecha = fecha

	qrB64, err := buildQR(params, 42, 71234567890123)
	if err != nil {
		t.Fatalf("no esperaba error, dio: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(qrB64)
	if err != nil {
		t.Fatalf("el QR no es base64 válido: %v", err)
	}

	var datos DatosQR
	if err := json.Unmarshal(raw, &datos); err != nil {
		t.Fatalf("el contenido del QR no es JSON válido: %v", err)
	}

	if datos.Ver != 1 {
		t.Errorf("ver = %d, esperaba 1 (versión fija exigida por AFIP)", datos.Ver)
	}
	if datos.Fecha != "2025-01-15" {
		t.Errorf("fecha = %q, esperaba %q", datos.Fecha, "2025-01-15")
	}
	if datos.CUIT != 20123456789 {
		t.Errorf("cuit = %d, esperaba %d", datos.CUIT, 20123456789)
	}
	if datos.NroCmp != 42 {
		t.Errorf("nroCmp = %d, esperaba 42", datos.NroCmp)
	}
	if datos.CodAut != 71234567890123 {
		t.Errorf("codAut = %d, esperaba el CAE 71234567890123", datos.CodAut)
	}
	if datos.TipoCodAut != "E" {
		t.Errorf("tipoCodAut = %q, esperaba %q (fijo, es CAE)", datos.TipoCodAut, "E")
	}
}

func TestEsMockMode(t *testing.T) {
	casos := []struct {
		nombre          string
		certPEM, keyPEM string
		env             string
		esperado        bool
	}{
		{"sin certs en testing", "", "", "testing", true},
		{"con certs en testing", "cert", "key", "testing", false},
		{"sin certs en producción nunca mockea", "", "", "produccion", false},
	}
	for _, c := range casos {
		if got := EsMockMode(c.certPEM, c.keyPEM, c.env); got != c.esperado {
			t.Errorf("%s: EsMockMode() = %v, esperaba %v", c.nombre, got, c.esperado)
		}
	}
}

func TestMockCAE_UsaElNumeroDeComprobantePedido(t *testing.T) {
	res := mockCAE(SolicitarCAEParams{NroComprobante: 42})
	if res.NroCmp != 42 {
		t.Errorf("NroCmp = %d, esperaba 42", res.NroCmp)
	}
	if res.CAE == "" {
		t.Error("el CAE mock no debería venir vacío")
	}
}
