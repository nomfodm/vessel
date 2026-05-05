import { useState } from 'react'
import { TitleBar } from './components/TitleBar/TitleBar'
import { TopNav } from './components/TopNav/TopNav'
import { StarCanvas } from './components/StarCanvas/StarCanvas'
import { LoginScreen } from './components/LoginScreen/LoginScreen'
import { PlayPage } from './components/PlayPage/PlayPage'
import { DetailPage } from './components/DetailPage/DetailPage'
import { InfoPage } from './components/InfoPage/InfoPage'
import type { Tab, User, Profile } from './types'
import styles from './App.module.css'

export function App() {
  const [user, setUser] = useState<User | null>(null)
  const [tab, setTab] = useState<Tab>('play')
  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null)

  function handleTabChange(newTab: Tab) {
    setTab(newTab)
    setSelectedProfile(null)
  }

  function handleSelectProfile(p: Profile) {
    setSelectedProfile(p)
    setTab('detail')
  }

  function handleLogout() {
    if (window.confirm('Выйти из аккаунта?')) setUser(null)
  }

  return (
    <div className={styles.app}>
      <StarCanvas />
      <TitleBar />
      {user ? (
        <>
          <TopNav tab={tab} setTab={handleTabChange} user={user} onLogout={handleLogout} />
          <div className={styles.content}>
            {tab === 'play' && !selectedProfile && <PlayPage onSelect={handleSelectProfile} />}
            {tab === 'detail' && selectedProfile && (
              <DetailPage
                profile={selectedProfile}
                onBack={() => { setTab('play'); setSelectedProfile(null) }}
              />
            )}
            {tab === 'info' && <InfoPage />}
          </div>
        </>
      ) : (
        <LoginScreen onLogin={setUser} />
      )}
    </div>
  )
}
