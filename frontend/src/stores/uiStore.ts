import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// Escalas de la pantalla de Caja. Se usa parada frente al mostrador y de reojo,
// así que hace falta poder agrandarla; el resto de las pantallas se usan
// sentado y de cerca y conviene que sigan entrando enteras.
//
// Hubo una escala "Más grande" (1.3) que se sacó: en la tablet real el ancho
// mínimo de las tres columnas de Caja (240 + 280 + 260 = 780) no entraba a
// ese zoom y el layout se rompía (carrito con ítems, filas apretadas contra
// los controles de cantidad). "Grande" sí entra, así que es el techo hasta
// no achicar esos mínimos y confirmar en la tablet real: al agregar una
// escala mayor, esos anchos mínimos de VentaPage tienen que seguir entrando
// en la pantalla más angosta que usa ese layout.
export const ESCALAS_CAJA = [
  { id: 'normal',     label: 'Normal',     valor: 1    },
  { id: 'grande',     label: 'Grande',     valor: 1.15 },
] as const

export type EscalaCajaID = typeof ESCALAS_CAJA[number]['id']

// Se persiste el ID, no el multiplicador: si algún día se ajusta cuánto
// agranda "Grande", las tablets que lo tenían elegido siguen en "Grande" en
// vez de quedar con un número huérfano.
//
// Recibe string y no EscalaCajaID porque lo persistido no está validado: si el
// localStorage quedó con un ID que ya no existe, cae a Normal. Tienen que
// pasar por acá TANTO el zoom como el selector, o el zoom caería a Normal
// mientras el selector muestra sus opciones apagadas.
export function escalaActiva(id: string) {
  return ESCALAS_CAJA.find(e => e.id === id) ?? ESCALAS_CAJA[0]
}

interface UIState {
  escalaCaja: EscalaCajaID
  setEscalaCaja: (id: EscalaCajaID) => void
}

// Preferencias del dispositivo, no de la cuenta: a propósito NO se limpian en
// resetSesion() del authStore. Cerrar sesión no tiene por qué achicarle la
// letra al que usa esta tablet. Mismo criterio que printerStore.
export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      escalaCaja: 'normal',
      setEscalaCaja: (id) => set({ escalaCaja: id }),
    }),
    { name: 'pos-ui' }
  )
)
