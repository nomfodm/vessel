import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { TitleBar } from './components/TitleBar/TitleBar'
import { TopNav } from './components/TopNav/TopNav'
import { StarCanvas } from './components/StarCanvas/StarCanvas'
import { LoginScreen } from './components/LoginScreen/LoginScreen'
import { PlayPage } from './components/PlayPage/PlayPage'
import { DetailPage } from './components/DetailPage/DetailPage'
import { InfoPage } from './components/InfoPage/InfoPage'
import type { Profile, Tab, User } from './types'
import styles from './App.module.css'

export function App() {
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

  function handleTabChange(newTab: Tab) {
    setTab(newTab)
    setSelectedProfile(null)
  }

  function handleSelectProfile(p: Profile) {
    setSelectedProfile(p)
    setTab('detail')
  }

  function handleLogout() {
    setUser(null)
  }

  return (
    <div className={styles.app}>
      <StarCanvas />
      <TitleBar />
      {user ? (
        <>
          <TopNav tab={tab} setTab={handleTabChange} user={user} onLogout={handleLogout} />
          <div className={styles.content}>
            {tab === 'play' && !selectedProfile && <PlayPage key="play" onSelect={handleSelectProfile} />}
            {tab === 'detail' && selectedProfile && (
              <DetailPage
                key={`detail-${selectedProfile.id}`}
                profile={selectedProfile}
                onBack={() => { setTab('play'); setSelectedProfile(null) }}
              />
            )}
            {tab === 'info' && <InfoPage key="info" />}
          </div>
        </>
      ) : (
        <LoginScreen onLogin={setUser} />
      )}
    </div>
  )
}
