package arca

import "encoding/xml"

// --- WSAA: Autenticación ---

type LoginTicketRequest struct {
	XMLName xml.Name  `xml:"loginTicketRequest"`
	Header  TRAHeader `xml:"header"`
	Service string    `xml:"service"`
}

type TRAHeader struct {
	UniqueID       string `xml:"uniqueId"`
	GenerationTime string `xml:"generationTime"`
	ExpirationTime string `xml:"expirationTime"`
}

type TicketAcceso struct {
	XMLName     xml.Name      `xml:"loginTicketResponse"`
	Header      TAHeader      `xml:"header"`
	Credentials TACredentials `xml:"credentials"`
}

type TAHeader struct {
	Source         string `xml:"source"`
	Destination    string `xml:"destination"`
	UniqueID       string `xml:"uniqueId"`
	GenerationTime string `xml:"generationTime"`
	ExpirationTime string `xml:"expirationTime"`
}

type TACredentials struct {
	Token string `xml:"token"`
	Sign  string `xml:"sign"`
}

// --- WSFE: Facturación electrónica ---

// Tipo de comprobante AFIP
const (
	TipoFacturaA = 1 // Factura A (empresa con CUIT)
	TipoFacturaB = 6 // Factura B (consumidor final)
)

// Tipo de documento receptor
const (
	TipoDocCUIT            = 80 // CUIT
	TipoDocConsumidorFinal = 99 // Sin identificar / consumidor final
)

// Datos para armar el QR AFIP
type DatosQR struct {
	Ver        int     `json:"ver"`
	Fecha      string  `json:"fecha"`
	CUIT       int64   `json:"cuit"`
	PtoVta     int     `json:"ptoVta"`
	TipoCmp    int     `json:"tipoCmp"`
	NroCmp     int64   `json:"nroCmp"`
	Importe    float64 `json:"importe"`
	Moneda     string  `json:"moneda"`
	Ctz        float64 `json:"ctz"`
	TipoDocRec int     `json:"tipoDocRec"`
	NroDocRec  int64   `json:"nroDocRec"`
	TipoCodAut string  `json:"tipoCodAut"`
	CodAut     int64   `json:"codAut"`
}
