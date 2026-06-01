import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { Icons } from '../Icons/Icons'
import { Button } from '../ui/Button/Button'
import { Badge } from '../ui/Badge/Badge'
import { Spinner } from '../ui/Spinner/Spinner'
import { ProgressBar } from '../ui/ProgressBar/ProgressBar'
import { SettingsSub } from './SettingsSub/SettingsSub'
import { OptFilesSub } from './OptFilesSub/OptFilesSub'
import { OPT_FILES } from '../../data/profiles'
import { Service as GameService } from '../../../bindings/github.com/nomfodm/vessel/internal/game'
import { ConfigService } from '../../../bindings/github.com/nomfodm/vessel/internal/config'
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

  // Load persisted per-profile settings from config.json.
  useEffect(() => {
    let cancelled = false
    ConfigService.Snapshot()
      .then(cfg => {
        if (cancelled) return
        const pc = cfg.profiles?.[profile.slug]
        if (!pc) return
        if (pc.ramMB) setRam(pc.ramMB)
        const enabled = new Set(pc.enabledOptional ?? [])
        setOptFiles(prev => prev.map(f => ({ ...f, on: enabled.has(f.id) })))
      })
      .catch(() => { /* keep defaults */ })
    return () => { cancelled = true }
  }, [profile.slug])

  function handleRamChange(next: number) {
    setRam(next)
    ConfigService.SetProfileRAM(profile.slug, next).catch(() => { /* ignore */ })
  }

  // Subscribe to backend game lifecycle events while this page is mounted.
  useEffect(() => {
    const offProgress = Events.On('sync:progress', (e: { data: { done: number; total: number } }) => {
      setGs('dl')
      setProg({ done: e.data.done, total: e.data.total })
    })
    const offStarted = Events.On('game:started', () => setGs('playing'))
    const offExited = Events.On('game:exited', () => {
      setGs('idle')
      setProg({ done: 0, total: 100 })
    })
    return () => { offProgress(); offStarted(); offExited() }
  }, [])

  function startPlay() {
    setGs('fetching')
    GameService.Launch(profile.slug).catch(() => {
      setGs('idle')
      setProg({ done: 0, total: 100 })
    })
  }

  function cancel() {
    GameService.Stop().catch(() => { /* ignore */ })
    setGs('idle')
    setProg({ done: 0, total: 100 })
  }

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
    setOptFiles(prev => {
      const next = prev.map(f => f.id === id ? { ...f, on: !f.on } : f)
      const enabled = next.filter(f => f.on).map(f => f.id)
      ConfigService.SetEnabledOptional(profile.slug, enabled).catch(() => { /* ignore */ })
      return next
    })
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
              {profile.status === 'online' && profile.players
                ? <Badge variant="online">{profile.players.online}/{profile.players.max}</Badge>
                : profile.status === 'offline'
                  ? <Badge variant="offline">Оффлайн</Badge>
                  : null}
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
        <SettingsSub ram={ram} path={path} onRamChange={handleRamChange} />
      )}

      {sub === 'optfiles' && (
        <OptFilesSub files={optFiles} onToggle={handleToggle} />
      )}
    </div>
  )
}
