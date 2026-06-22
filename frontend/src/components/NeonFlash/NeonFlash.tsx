import { useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import styles from './NeonFlash.module.css'

// Launcher palette (cyan/violet/indigo) pushed into bolder neons: fuchsia,
// magenta, hot pink, electric blue — "дерзкие" but still on-brand.
const COLORS = [
  '37,195,232',  // cyan
  '43,107,255',  // electric blue
  '139,92,246',  // violet
  '217,70,239',  // fuchsia
  '255,43,214',  // magenta
  '255,32,110',  // hot pink
  '86,37,232',   // indigo
]

const W = 960
const H = 600
const MAX = 10

type Flash = { x: number; y: number; r: number; color: string; t: number; life: number; intensity: number }

function smooth(x: number): number {
  x = x < 0 ? 0 : x > 1 ? 1 : x
  return x * x * (3 - 2 * x)
}

// Brightness over life: slow rise -> bright hold -> long slow fade.
function envelope(p: number): number {
  if (p < 0.25) return smooth(p / 0.25)
  if (p < 0.42) return 1
  return smooth(1 - (p - 0.42) / 0.58)
}

export function NeonFlash() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')!
    canvas.width = W
    canvas.height = H

    const flashes: Flash[] = []

    function spawn() {
      flashes.push({
        x: Math.random() * W,
        y: Math.random() * H,
        r: 130 + Math.random() * 200, // smaller core; the blur smears it wide
        color: COLORS[(Math.random() * COLORS.length) | 0],
        t: 0,
        life: 200 + Math.random() * 220, // ~3.5–7s: slow rise, long fade
        intensity: 0.75 + Math.random() * 0.6, // shine very bright
      })
    }

    for (let i = 0; i < 3; i++) spawn() // seed so it isn't empty on first paint

    let raf: number
    let paused = false

    function draw() {
      if (paused) return
      ctx.clearRect(0, 0, W, H)
      ctx.globalCompositeOperation = 'lighter'
      ctx.filter = 'blur(60px)' // smear every bloom into edgeless haze

      if (flashes.length < MAX && Math.random() < 0.025) spawn() // appear slowly

      for (let i = flashes.length - 1; i >= 0; i--) {
        const f = flashes[i]
        f.t++
        const phase = f.t / f.life
        if (phase >= 1) {
          flashes.splice(i, 1)
          continue
        }
        const a = envelope(phase) * f.intensity
        const r = f.r * (0.6 + 0.4 * phase) // slow bloom outward
        // Soft, edgeless falloff — no visible circle outline.
        const grad = ctx.createRadialGradient(f.x, f.y, 0, f.x, f.y, r)
        grad.addColorStop(0, `rgba(${f.color},${a})`)
        grad.addColorStop(0.45, `rgba(${f.color},${a * 0.16})`)
        grad.addColorStop(1, `rgba(${f.color},0)`)
        ctx.fillStyle = grad
        ctx.beginPath()
        ctx.arc(f.x, f.y, r, 0, Math.PI * 2)
        ctx.fill()
      }

      ctx.filter = 'none'
      ctx.globalCompositeOperation = 'source-over'
      raf = requestAnimationFrame(draw)
    }

    const pause = () => { paused = true; cancelAnimationFrame(raf) }
    const resume = () => { if (!paused) return; paused = false; draw() }

    const off1 = Events.On('common:WindowMinimise', pause)
    const off2 = Events.On('common:WindowRestore', resume)
    const off3 = Events.On('common:WindowLostFocus', pause)
    const off4 = Events.On('common:WindowFocus', resume)

    draw()
    return () => {
      cancelAnimationFrame(raf)
      off1(); off2(); off3(); off4()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className={styles.canvas}
    />
  )
}
