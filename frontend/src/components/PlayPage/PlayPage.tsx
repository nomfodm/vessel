import { useState, useEffect } from 'react'
import { Badge } from '../ui/Badge/Badge'
import { Spinner } from '../ui/Spinner/Spinner'
import { Service as ProfileService } from '../../../bindings/github.com/nomfodm/vessel/internal/profile'
import type { Profile } from '../../types'
import styles from './PlayPage.module.css'

interface PlayPageProps {
  onSelect: (profile: Profile) => void
}

export function PlayPage({ onSelect }: PlayPageProps) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')

  useEffect(() => {
    let cancelled = false
    ProfileService.Profiles()
      .then(list => { if (!cancelled) setProfiles(list as Profile[]) })
      .catch(() => { if (!cancelled) setErr('Не удалось загрузить список режимов') })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.header}>
          <div className={styles.title}>Выберите режим</div>
          <div className={styles.subtitle}><Spinner /> Загрузка...</div>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.title}>Выберите режим</div>
        <div className={styles.subtitle}>{err || `Доступно режимов: ${profiles.length}`}</div>
      </div>
      <div className={styles.grid}>
        {profiles.map(p => {
          return (
            <div
              key={p.slug}
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
                  {p.status === 'online' && p.players && <Badge variant="online">{p.players.online}/{p.players.max}</Badge>}
                  {p.status === 'offline' && <Badge variant="offline">Оффлайн</Badge>}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
