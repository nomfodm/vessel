import { useState, useEffect } from 'react'
import { Badge } from '../ui/Badge/Badge'
import { PROFILES } from '../../data/profiles'
import type { Profile } from '../../types'
import styles from './PlayPage.module.css'

type PingStatus = 'pinging' | 'online' | 'offline'

interface PlayPageProps {
  onSelect: (profile: Profile) => void
}

export function PlayPage({ onSelect }: PlayPageProps) {
  const [pings, setPings] = useState<Record<number, PingStatus>>({ 1: 'pinging', 2: 'pinging', 3: 'pinging' })

  useEffect(() => {
    PROFILES.forEach((p, i) => {
      setTimeout(() => setPings(prev => ({ ...prev, [p.id]: p.status })), 500 + i * 350)
    })
  }, [])

  const onlineCount = PROFILES.filter(p => p.status === 'online').length

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.title}>Выберите режим</div>
        <div className={styles.subtitle}>{onlineCount} из {PROFILES.length} серверов онлайн</div>
      </div>
      <div className={styles.grid}>
        {PROFILES.map(p => {
          const ps = pings[p.id]
          return (
            <div
              key={p.id}
              className={styles.card}
              style={{ background: p.bg }}
              onClick={() => onSelect(p)}
              onMouseEnter={e => {
                const el = e.currentTarget as HTMLDivElement
                el.style.transform = 'translateY(-4px)'
                el.style.boxShadow = `0 20px 50px rgba(0,0,0,0.5), 0 0 0 1px ${p.accent}33`
              }}
              onMouseLeave={e => {
                const el = e.currentTarget as HTMLDivElement
                el.style.transform = ''
                el.style.boxShadow = ''
              }}
            >
              <div className={styles.accentLine} style={{ background: `linear-gradient(90deg, ${p.accent}, transparent)` }} />
              <div className={styles.cardBody}>
                <div className={styles.icon}>{p.icon}</div>
                <div className={styles.cardTitle}>{p.title}</div>
                <div className={styles.cardDesc}>{p.desc.slice(0, 60)}...</div>
                <div className={styles.badges}>
                  <Badge variant="version">{p.version}</Badge>
                  {ps === 'pinging' && <Badge variant="pinging">Пинг...</Badge>}
                  {ps === 'online' && <Badge variant="online">{p.players.online}/{p.players.max}</Badge>}
                  {ps === 'offline' && <Badge variant="offline">Оффлайн</Badge>}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
