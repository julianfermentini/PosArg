package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"pos-fiscal/internal/models"
)

// AuthRequired valida la firma/expiración del JWT y además compara su claim
// "tv" (token_version) contra el vigente en la base. Sin este segundo chequeo,
// cambiar la contraseña no invalidaría los JWT ya emitidos: seguirían sirviendo
// hasta que expiren (30 días) aunque la cuenta ya se haya "asegurado". Un token
// viejo sin claim "tv" decodifica como 0, que es el default de la columna, así
// que los tokens emitidos antes de este cambio siguen funcionando sin forzar
// un logout masivo.
func AuthRequired(secret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token requerido"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token inválido o expirado"})
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)

		empresaIDStr, _ := claims["empresa_id"].(string)
		if _, err := uuid.Parse(empresaIDStr); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token inválido — re-iniciá sesión"})
			return
		}

		subStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(subStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token inválido — re-iniciá sesión"})
			return
		}

		// Los números de un JSON decodificado a interface{} llegan como float64.
		tokenVersion, _ := claims["tv"].(float64)

		// First (no Pluck) para que un usuario borrado dé gorm.ErrRecordNotFound
		// en vez de un "vigente" en cero indistinguible de un token nunca
		// invalidado — con Pluck, la cuenta eliminada de un admin seguiría
		// autenticando mientras el JWT no haya expirado.
		var user models.User
		if err := db.WithContext(c.Request.Context()).Select("token_version").
			First(&user, "id = ?", userID).Error; err != nil || user.TokenVersion != int(tokenVersion) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token inválido — re-iniciá sesión"})
			return
		}

		c.Set("user_id", claims["sub"])
		c.Set("email", claims["email"])
		c.Set("negocio_nombre", claims["negocio_nombre"])
		c.Set("empresa_id", empresaIDStr)
		c.Next()
	}
}
