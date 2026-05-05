import { useEffect, useRef } from 'react'

export function StarCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')!
    const W = 960
    const H = 600
    canvas.width = W
    canvas.height = H

    const stars: Array<{ x: number; y: number; r: number; o: number; t: number; ts: number }> = []
    const particles: Array<{ x: number; y: number; vx: number; vy: number; r: number; color: string; o: number }> = []

    for (let i = 0; i < 180; i++) {
      stars.push({
        x: Math.random() * W,
        y: Math.random() * H,
        r: Math.random() * 1.2 + 0.2,
        o: Math.random() * 0.5 + 0.1,
        t: Math.random() * Math.PI * 2,
        ts: (Math.random() - 0.5) * 0.012,
      })
    }
    for (let i = 0; i < 28; i++) {
      particles.push({
        x: Math.random() * W,
        y: Math.random() * H,
        vx: (Math.random() - 0.5) * 0.25,
        vy: (Math.random() - 0.5) * 0.15 - 0.05,
        r: Math.random() * 2 + 0.8,
        color: Math.random() > 0.5 ? 'rgba(37,195,232,' : 'rgba(86,37,232,',
        o: Math.random() * 0.35 + 0.1,
      })
    }

    let raf: number

    function draw() {
      ctx.clearRect(0, 0, W, H)

      const g1 = ctx.createRadialGradient(W * 0.25, H * 0.3, 0, W * 0.25, H * 0.3, 280)
      g1.addColorStop(0, 'rgba(37,195,232,0.07)')
      g1.addColorStop(1, 'transparent')
      ctx.fillStyle = g1
      ctx.fillRect(0, 0, W, H)

      const g2 = ctx.createRadialGradient(W * 0.78, H * 0.72, 0, W * 0.78, H * 0.72, 300)
      g2.addColorStop(0, 'rgba(86,37,232,0.08)')
      g2.addColorStop(1, 'transparent')
      ctx.fillStyle = g2
      ctx.fillRect(0, 0, W, H)

      stars.forEach(s => {
        s.t += s.ts
        const o = s.o + Math.sin(s.t) * 0.25
        ctx.beginPath()
        ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(210,210,255,${Math.max(0, o)})`
        ctx.fill()
      })

      particles.forEach(p => {
        p.x += p.vx
        p.y += p.vy
        if (p.x < 0) p.x = W
        if (p.x > W) p.x = 0
        if (p.y < 0) p.y = H
        if (p.y > H) p.y = 0
        const pg = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.r * 3)
        pg.addColorStop(0, p.color + p.o + ')')
        pg.addColorStop(1, p.color + '0)')
        ctx.beginPath()
        ctx.arc(p.x, p.y, p.r * 3, 0, Math.PI * 2)
        ctx.fillStyle = pg
        ctx.fill()
      })

      raf = requestAnimationFrame(draw)
    }

    draw()
    return () => cancelAnimationFrame(raf)
  }, [])

  return (
    <canvas
      ref={canvasRef}
      style={{ position: 'absolute', inset: 0, zIndex: 0, pointerEvents: 'none' }}
    />
  )
}
