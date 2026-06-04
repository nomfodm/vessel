import { useState, useEffect, useCallback } from 'react'
import { Events } from '@wailsio/runtime'
import { Service as AuthService } from '../bindings/github.com/nomfodm/vessel/internal/auth'
import { Service as HealthService } from '../bindings/github.com/nomfodm/vessel/internal/health'
import { TitleBar } from './components/TitleBar/TitleBar'
import { TopNav } from './components/TopNav/TopNav'
import { StarCanvas } from './components/StarCanvas/StarCanvas'
import { LoginScreen } from './components/LoginScreen/LoginScreen'
import { ErrorScreen } from './components/ErrorScreen/ErrorScreen'
import { BootScreen } from './components/BootScreen/BootScreen'
import { PlayPage } from './components/PlayPage/PlayPage'
import { DetailPage } from './components/DetailPage/DetailPage'
import { InfoPage } from './components/InfoPage/InfoPage'
import type { Profile, Tab, User } from './types'
import styles from './App.module.css'

type Phase = 'checking' | 'offline' | 'ready'
type ServerStatus = { name: string; ok: boolean; state: string }
type OfflineReason = 'offline' | 'maintenance' | 'off'

export function App() {
  const [phase, setPhase] = useState<Phase>('checking')
  const [statuses, setStatuses] = useState<ServerStatus[]>([])
  const [reason, setReason] = useState<OfflineReason>('offline')
  const [retrying, setRetrying] = useState(false)
  const [booting, setBooting] = useState(true)
  const [user, setUser] = useState<User | null>(null)
  const [tab, setTab] = useState<Tab>('play')
  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null)

  useEffect(() => {
    const pause = () => document.documentElement.classList.add('paused')
    const resume = () => document.documentElement.classList.remove('paused')
    const off1 = Events.On('common:WindowMinimise', pause)
    const off2 = Events.On('common:WindowRestore', resume)
    const off3 = Events.On('common:WindowLostFocus', pause)
    const off4 = Events.On('common:WindowFocus', resume)
    return () => { off1(); off2(); off3(); off4() }
  }, [])

  // Startup: probe required servers first. Only on success do we touch auth —
  // a session restore against a dead API would just hang. On failure we stop at
  // the offline screen. Reused by the retry button.
  const startup = useCallback(async () => {
    setRetrying(true)
    let report
    try {
      report = await HealthService.Check()
    } catch {
      setStatuses([])
      setReason('offline')
      setPhase('offline')
      setRetrying(false)
      return
    }
    const list = (report.statuses ?? []) as ServerStatus[]
    setStatuses(list)
    if (!report.ok) {
      const states = list.map(s => s.state)
      setReason(states.includes('maintenance') ? 'maintenance' : states.includes('off') ? 'off' : 'offline')
      setPhase('offline')
      setRetrying(false)
      return
    }
    setPhase('ready')
    setRetrying(false)

    try {
      // Frontend reload doesn't restart the Go process: if the service is
      // still logged in, reuse the session without a network refresh.
      // Restore() (with its network refresh) is only for a cold start.
      const live = await AuthService.IsLoggedIn()
      const ok = live || await AuthService.Restore()
      if (ok) {
        const u = await AuthService.CurrentUser()
        setUser({ username: u.username })
      }
    } catch {
      /* no session — show login */
    } finally {
      setBooting(false)
    }
  }, [])

  useEffect(() => { void startup() }, [startup])

  // Keep the boot overlay mounted briefly after loading ends so its fade-out can
  // play before it leaves the DOM.
  const loading = phase === 'checking' || booting
  const [bootMounted, setBootMounted] = useState(true)
  useEffect(() => {
    if (loading) { setBootMounted(true); return }
    const t = setTimeout(() => setBootMounted(false), 380)
    return () => clearTimeout(t)
  }, [loading])

  function handleTabChange(newTab: Tab) {
    setTab(newTab)
    setSelectedProfile(null)
  }

  function handleSelectProfile(p: Profile) {
    setSelectedProfile(p)
    setTab('detail')
  }

  function handleLogout() {
    AuthService.Logout().catch(() => { /* clear locally regardless */ })
    setUser(null)
  }

  if (phase === 'offline') {
    return (
      <div className={styles.app}>
        <TitleBar />
        <ErrorScreen statuses={statuses} reason={reason} retrying={retrying} onRetry={startup} />
      </div>
    )
  }

  return (
    <div className={styles.app}>
      {!loading && <StarCanvas />}
      <TitleBar />
      {!loading && (user ? (
        <>
          <TopNav tab={tab} setTab={handleTabChange} user={user} onLogout={handleLogout} />
          <div className={styles.content}>
            {tab === 'play' && !selectedProfile && <PlayPage key="play" onSelect={handleSelectProfile} />}
            {tab === 'detail' && selectedProfile && (
              <DetailPage
                key={`detail-${selectedProfile.slug}`}
                profile={selectedProfile}
                onBack={() => { setTab('play'); setSelectedProfile(null) }}
              />
            )}
            {tab === 'info' && <InfoPage key="info" />}
          </div>
        </>
      ) : (
        <LoginScreen onLogin={setUser} />
      ))}
      {bootMounted && (
        <BootScreen
          message={phase === 'checking' ? 'Подключение к серверам…' : 'Загрузка…'}
          closing={!loading}
        />
      )}
    </div>
  )
}
