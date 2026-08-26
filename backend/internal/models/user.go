package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EmpresaID    uuid.UUID `gorm:"type:uuid;index" json:"empresa_id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	// TokenVersion viaja en el claim "tv" del JWT. Cambiar la contraseña (propia
	// o vía reset de admin) la incrementa, así que cualquier token emitido antes
	// deja de servir en el acto — sin esto, un JWT filtrado seguía siendo válido
	// hasta por 30 días aunque la cuenta ya se hubiera "asegurado".
	TokenVersion  int       `gorm:"not null;default:0" json:"-"`
	NegocioNombre string    `gorm:"not null" json:"negocio_nombre"`
	CreatedAt     time.Time `json:"created_at"`
}

func (User) TableName() string { return "users" }
