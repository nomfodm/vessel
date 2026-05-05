import { useState, useRef, useEffect } from 'react'
import { Icons } from '../Icons/Icons'
import { Button } from '../ui/Button/Button'
import { Badge } from '../ui/Badge/Badge'
import { Spinner } from '../ui/Spinner/Spinner'
import { ProgressBar } from '../ui/ProgressBar/ProgressBar'
import { SettingsSub } from './SettingsSub/SettingsSub'
import { OptFilesSub } from './OptFilesSub/OptFilesSub'
import { OPT_FILES } from '../../data/profiles'
import type { Profile, OptFile, GameState } from '../../types'
import styles from './DetailPage.module.css'

type DetailSub = 'main' | 'settings' | 'optfiles'

interface DetailPageProps {
  profile: Profile
  onBack: () => void
}

export function DetailPage({ profile, onBack }: DetailPageProps) {
  const [sub, setSub] = useState<DetailSub>('main')
  const [gs, setGs] = useState<GameState>('idle')
  const [prog, setProg] = useState({ done: 0, total: 100 })
  const [ram, setRam] = useState(4096)
  const [path] = useState(`C:\\Users\\Player\\AppData\\Roaming\\.infinity\\${profile.slug}`)
  const [optFiles, setOptFiles] = useState<OptFile[]>(OPT_FILES)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  function startPlay() {
    setGs('fetching')
    setTimeout(() => {
      setGs('dl')
      setProg({ done: 0, total: 312 })
      timerRef.current = setInterval(() => {
        setProg(p => {
          const next = Math.min(p.done + Math.floor(Math.random() * 15) + 5, p.total)
          if (next >= p.total) {
            clearInterval(timerRef.current!)
            setGs('prep')
            setTimeout(() => setGs('playing'), 1100)
            return { ...p, done: p.total }
          }
          return { ...p, done: next }
        })
      }, 140)
    }, 800)
  }

  function cancel() {
    if (timerRef.current) clearInterval(timerRef.current)
    setGs('idle')
    setProg({ done: 0, total: 100 })
  }

  useEffect(() => () => { if (timerRef.current) clearInterval(timerRef.current) }, [])

  const isIdle = gs === 'idle'
  const isLoading = gs === 'fetching' || gs === 'dl' || gs === 'prep'
  const isPlaying = gs === 'playing'

  const statusMsg: Record<GameState, string> = {
    fetching: 'Получаю файлы игры...',
    dl: `Загрузка ${prog.done} / ${prog.total} МБ`,
    prep: 'Подготовка к запуску...',
    playing: 'Игра запущена',
    idle: '',
  }

  function handleToggle(id: string) {
    setOptFiles(prev => prev.map(f => f.id === id ? { ...f, on: !f.on } : f))
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>
        <button
          className={styles.backBtn}
          onClick={onBack}
          onMouseEnter={e => { (e.currentTarget as HTMLElement).style.color = 'var(--cyan)' }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.color = 'var(--text-dim)' }}
        >
          <Icons.Back /> Режимы
        </button>
        <span className={styles.separator}>›</span>
        {sub !== 'main' ? (
          <>
            <span className={styles.crumbLink} onClick={() => setSub('main')}>{profile.title}</span>
            <span className={styles.separator}>›</span>
            <span className={styles.crumbActive}>
              {sub === 'settings' ? 'Настройки' : 'Опциональные файлы'}
            </span>
          </>
        ) : (
          <span className={styles.crumbActive}>{profile.title}</span>
        )}
      </div>

      {sub === 'main' && (
        <div className={styles.mainSub}>
          <div
            className={styles.banner}
            style={{ background: profile.bg, border: `1px solid ${profile.accent}22` }}
          >
            <div className={styles.bannerIcon}>{profile.icon}</div>
            <div className={styles.bannerBadges}>
              <Badge variant="version">{profile.version}</Badge>
              {profile.status === 'online'
                ? <Badge variant="online">{profile.players.online}/{profile.players.max}</Badge>
                : <Badge variant="offline">Оффлайн</Badge>}
            </div>
            <div className={styles.bannerTitle}>{profile.title}</div>
            <div className={styles.bannerDesc}>{profile.desc}</div>
          </div>

          <div>
            {statusMsg[gs] && (
              <div className={styles.status}>
                {isLoading && <Spinner />}
                {isPlaying && <span style={{ color: profile.accent }}>●</span>}
                {statusMsg[gs]}
              </div>
            )}
            {gs === 'dl' && (
              <div className={styles.progressWrap}>
                <ProgressBar value={(prog.done / prog.total) * 100} />
              </div>
            )}
            <div className={styles.actionButtons}>
              {isIdle && (
                <Button variant="primary" style={{ minWidth: 120 }} onClick={startPlay}>
                  ▶ Играть
                </Button>
              )}
              {isLoading && (
                <Button variant="danger" style={{ minWidth: 120 }} onClick={cancel}>
                  ✕ Отмена
                </Button>
              )}
              {isPlaying && (
                <Button variant="danger" style={{ minWidth: 140 }} onClick={cancel}>
                  ■ Закрыть игру
                </Button>
              )}
              {isIdle && (
                <>
                  <Button variant="icon-sq" onClick={() => setSub('settings')} title="Настройки">
                    <Icons.Settings />
                  </Button>
                  <Button variant="icon-sq" onClick={() => setSub('optfiles')} title="Моды / ресурспаки">
                    <Icons.Files />
                  </Button>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {sub === 'settings' && (
        <SettingsSub ram={ram} path={path} onRamChange={setRam} />
      )}

      {sub === 'optfiles' && (
        <OptFilesSub files={optFiles} onToggle={handleToggle} />
      )}
    </div>
  )
}
