import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import {
  conectarUSB, imprimirUSB, desconectarUSB,
  conectarBluetooth, imprimirBluetooth, desconectarBluetooth,
  buildTicketBytes, buildTicketNoFiscalBytes, buildCierreBytes, buildRotuloBytes,
  type DatosTicketFront, type DatosTicketNoFiscal, type DatosCierre, type DatosRotulo,
} from '../lib/printer'

type TipoConexion = 'usb' | 'bluetooth' | null

// Devuelve null si la impresión salió, o el mensaje de error si no. Mismo
// contrato que rotulosStore: el caller decide qué hacer (limpiar el carrito,
// cantar un ✓) con lo que se le devolvió, en vez de tener que acordarse de ir a
// leer el estado después — que es fácil de olvidar y ya había dejado a dos
// pantallas festejando impresiones que nunca salieron.
type Impresion = Promise<string | null>

export interface PrinterStore {
  tipo:      TipoConexion
  nombre:    string | null
  conectado: boolean
  error:     string | null

  conectarUSB:       () => Promise<void>
  conectarBluetooth: () => Promise<void>
  desconectar:       () => void
  imprimir:          (datos: DatosTicketFront) => Impresion
  imprimirNoFiscal:  (datos: DatosTicketNoFiscal) => Impresion
  imprimirCierre:    (datos: DatosCierre) => Impresion
  imprimirRotulo:    (datos: DatosRotulo) => Impresion
  clearError:        () => void
}

export const usePrinterStore = create<PrinterStore>()(
  persist(
    (set, get) => {
      // Único lugar que habla con el transporte. Las cuatro impresiones sólo se
      // diferencian en qué bytes arman, así que el resto vive acá una sola vez.
      // `armar` va como función y no como bytes ya hechos para que un error del
      // builder caiga dentro del try, como caía antes.
      const enviar = async (armar: () => Uint8Array, siFalla: string): Impresion => {
        const { tipo, conectado } = get()
        if (!conectado || !tipo) return 'Sin impresora conectada'
        try {
          const bytes = armar()
          if (tipo === 'usb') await imprimirUSB(bytes)
          else                await imprimirBluetooth(bytes)
          set({ error: null })
          return null
        } catch (e: any) {
          const msg = e.message ?? siFalla
          set({ conectado: false, error: msg })
          return msg
        }
      }

      return {
        tipo:      null,
        nombre:    null,
        conectado: false,
        error:     null,

        conectarUSB: async () => {
          try {
            const info = await conectarUSB()
            set({ tipo: 'usb', nombre: info.nombre, conectado: true, error: null })
          } catch (e: any) {
            set({ error: e.message ?? 'Error conectando impresora USB' })
          }
        },

        conectarBluetooth: async () => {
          try {
            const info = await conectarBluetooth()
            set({ tipo: 'bluetooth', nombre: info.nombre, conectado: true, error: null })
          } catch (e: any) {
            set({ error: e.message ?? 'Error conectando impresora Bluetooth' })
          }
        },

        desconectar: () => {
          const { tipo } = get()
          if (tipo === 'usb')       desconectarUSB()
          if (tipo === 'bluetooth') desconectarBluetooth()
          set({ tipo: null, nombre: null, conectado: false, error: null })
        },

        imprimir:         (datos) => enviar(() => buildTicketBytes(datos),         'Error al imprimir'),
        imprimirNoFiscal: (datos) => enviar(() => buildTicketNoFiscalBytes(datos), 'Error al imprimir'),
        imprimirCierre:   (datos) => enviar(() => buildCierreBytes(datos),         'Error al imprimir cierre'),
        imprimirRotulo:   (datos) => enviar(() => buildRotuloBytes(datos),         'Error al imprimir rótulo'),

        clearError: () => set({ error: null }),
      }
    },
    {
      name: 'pos-printer',
      // Solo persistir el tipo y nombre para mostrar el estado entre recargas.
      // El objeto del dispositivo vive en la memoria del módulo printer.ts y se pierde al recargar.
      partialize: (s) => ({ tipo: s.tipo, nombre: s.nombre }),
    }
  )
)
