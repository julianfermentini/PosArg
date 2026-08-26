package handlers

import (
	"sync"

	"github.com/google/uuid"
)

// keyedMutex da un lock independiente por clave (una por empresa, en el caso
// de Worker.caeLocks), para serializar las llamadas a ARCA dentro de una
// misma empresa sin bloquear a las demás. Antes había un único sync.Mutex
// global para todo el proceso: con multi-tenant real, eso hacía que el CAE
// lento de una empresa (hasta timeoutCAE) frenara a todas las demás.
//
// Las entradas del mapa quedan para siempre — nunca se borran. Con la escala
// esperada (un puñado de empresas por proceso) el mapa no crece lo bastante
// como para que valga la pena limpiarlo.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

// Lock toma el lock de key (creándolo si es la primera vez) y devuelve la
// función para liberarlo — se usa como `defer keyedMutex.Lock(id)()`.
func (k *keyedMutex) Lock(key uuid.UUID) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[uuid.UUID]*sync.Mutex)
	}
	l, ok := k.locks[key]
	if !ok {
		l = &sync.Mutex{}
		k.locks[key] = l
	}
	k.mu.Unlock()

	l.Lock()
	return l.Unlock
}
