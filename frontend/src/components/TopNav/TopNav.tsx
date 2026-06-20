import { useState, useRef, useEffect } from 'react'
import { Logo } from '../Logo/Logo'
import { Icons } from '../Icons/Icons'
import { cn } from '../../utils/cn'
import type { Tab, User } from '../../types'
import styles from './TopNav.module.css'

interface TopNavProps {
  tab: Tab
  setTab: (tab: Tab) => void
  user: User
  onLogout: () => void
  syncing?: boolean
  locked?: boolean
}

export function TopNav({ tab, setTab, user, onLogout, syncing, locked }: TopNavProps) {
  const activeTab = tab === 'detail' ? 'play' : tab
  const [open, setOpen] = useState(false)
  const userRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (userRef.current && !userRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div className={styles.topnav}>
      <div className={styles.logo}>
        <Logo size={26} />
        <span className={styles.logoText}>Infinity</span>
      </div>
      <div className={styles.divider} />
      <button
        className={cn(styles.tab, activeTab === 'play' && styles.active)}
        disabled={locked}
        onClick={() => setTab('play')}
      >
        <Icons.Play /> Играть
      </button>
      <button
        className={cn(styles.tab, activeTab === 'info' && styles.active)}
        disabled={locked}
        onClick={() => setTab('info')}
      >
        <Icons.Info /> О программе
      </button>
      <button
        className={cn(styles.tab, activeTab === 'settings' && styles.active)}
        disabled={locked}
        onClick={() => setTab('settings')}
      >
        <Icons.Settings /> Настройки
      </button>
      <div className={styles.right}>
        <div ref={userRef} className={styles.userWrap}>
          <div
            className={cn(styles.user, open && styles.userActive)}
            onClick={() => setOpen(o => !o)}
          >
            <div className={styles.avatar}>
              {user.avatarUrl
                ? <img src={user.avatarUrl} alt={user.username} className={styles.avatarImg} />
                : user.username[0].toUpperCase()
              }
            </div>
            <span className={styles.username}>{user.username}</span>
          </div>
          {open && (
            <div className={styles.dropdown}>
              <button
                className={styles.dropdownItem}
                disabled={syncing || locked}
                onClick={() => { if (!syncing && !locked) { setOpen(false); onLogout() } }}
                title={locked ? 'Дождитесь окончания переноса' : syncing ? 'Дождитесь окончания загрузки' : undefined}
              >
                {locked ? 'Перенос…' : syncing ? 'Загрузка...' : 'Выйти'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
