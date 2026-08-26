package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"pos-fiscal/internal/models"
)

type AuthHandler struct {
	db         *gorm.DB
	jwtSecret  string
	inviteCode string // vacío = modo legacy (solo primer usuario)
}

// hashDummy es un hash bcrypt de una contraseña que nadie tipeó, calculado una
// sola vez al arrancar. Login lo usa cuando el email no existe, para que
// CompareHashAndPassword corra igual que en el camino real: sin esto, un
// intento con email inexistente responde notoriamente más rápido que uno con
// email válido y contraseña incorrecta (no hay bcrypt de por medio), lo que
// deja medir por tiempo qué emails están registrados.
var hashDummy = mustHashDummy()

func mustHashDummy() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("posarg-dummy-password-timing-safety"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
}

func NuevoAuthHandler(db *gorm.DB, jwtSecret, inviteCode string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, inviteCode: inviteCode}
}

type RegisterRequest struct {
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=6"`
	NegocioNombre string `json:"negocio_nombre" binding:"required"`
	InviteCode    string `json:"invite_code"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register crea una nueva empresa+usuario. Con INVITE_CODE configurado, acepta
// múltiples registros (multitenant SaaS). Sin él, solo permite el primero.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if h.inviteCode != "" {
		if req.InviteCode != h.inviteCode {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Código de invitación inválido"})
			return
		}
	} else {
		// Modo legacy: solo el primer usuario puede registrarse
		var count int64
		h.db.WithContext(c.Request.Context()).Model(&models.User{}).Count(&count)
		if count > 0 {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Ya existe una cuenta registrada"})
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno"})
		return
	}

	user, err := crearEmpresaConUsuario(c.Request.Context(), h.db, req.Email, string(hash), req.NegocioNombre)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error creando cuenta"})
		return
	}

	token, err := generarToken(user, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error generando token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"token":          token,
			"email":          user.Email,
			"negocio_nombre": user.NegocioNombre,
		},
	})
}

// Login valida credenciales y devuelve JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.WithContext(c.Request.Context()).Where("email = ?", req.Email).First(&user).Error; err != nil {
		bcrypt.CompareHashAndPassword(hashDummy, []byte(req.Password))
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Email o contraseña incorrectos"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Email o contraseña incorrectos"})
		return
	}

	var activo bool
	if err := h.db.WithContext(c.Request.Context()).Model(&models.ConfigEmpresa{}).
		Select("activo").Where("id = ?", user.EmpresaID).Scan(&activo).Error; err != nil || !activo {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Cuenta suspendida. Contactá al administrador."})
		return
	}

	token, err := generarToken(user, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error generando token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token":          token,
			"email":          user.Email,
			"negocio_nombre": user.NegocioNombre,
		},
	})
}

type CambiarPasswordRequest struct {
	PasswordActual string `json:"password_actual" binding:"required"`
	PasswordNueva  string `json:"password_nueva" binding:"required,min=6"`
}

// CambiarPassword permite a un usuario logueado cambiar su propia contraseña,
// verificando primero la actual. Distinto del reset de admin (que no la pide).
func (h *AuthHandler) CambiarPassword(c *gin.Context) {
	var req CambiarPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.WithContext(c.Request.Context()).First(&user, "id = ?", getUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Usuario no encontrado"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.PasswordActual)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "La contraseña actual es incorrecta"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.PasswordNueva), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno"})
		return
	}

	// token_version += 1 invalida cualquier JWT emitido antes de este cambio —
	// incluido el de esta misma sesión, por eso reemitimos uno nuevo abajo en
	// vez de dejar que este pedido termine deslogueando a quien acaba de
	// cambiar su propia contraseña.
	if err := h.db.WithContext(c.Request.Context()).Model(&user).Updates(map[string]interface{}{
		"password_hash": string(hash),
		"token_version": gorm.Expr("token_version + 1"),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudo actualizar la contraseña"})
		return
	}
	user.TokenVersion++

	token, err := generarToken(user, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error generando token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"token": token}})
}

// HasUsers indica si hay usuarios y si el registro por invitación está habilitado.
func (h *AuthHandler) HasUsers(c *gin.Context) {
	var count int64
	h.db.WithContext(c.Request.Context()).Model(&models.User{}).Count(&count)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"has_users":      count > 0,
			"invite_enabled": h.inviteCode != "",
		},
	})
}

// crearEmpresaConUsuario crea una ConfigEmpresa y su User asociado en una
// transacción atómica. Es la única fuente de verdad para crear cuentas nuevas.
func crearEmpresaConUsuario(ctx context.Context, db *gorm.DB, email, passwordHash, negocioNombre string) (models.User, error) {
	var user models.User
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		empresa := models.ConfigEmpresa{
			ID:           uuid.New(),
			RazonSocial:  negocioNombre,
			PuntoVenta:   1,
			ArcaEnv:      "testing",
			CondicionIVA: "Responsable Inscripto",
		}
		if err := tx.Create(&empresa).Error; err != nil {
			return err
		}
		user = models.User{
			EmpresaID:     empresa.ID,
			Email:         email,
			PasswordHash:  passwordHash,
			NegocioNombre: negocioNombre,
		}
		return tx.Create(&user).Error
	})
	return user, err
}

func generarToken(user models.User, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":            user.ID.String(),
		"empresa_id":     user.EmpresaID.String(),
		"email":          user.Email,
		"negocio_nombre": user.NegocioNombre,
		"tv":             user.TokenVersion,
		"exp":            time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
