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
}

export function TopNav({ tab, setTab, user, onLogout }: TopNavProps) {
  const activeTab = tab === 'detail' ? 'play' : tab

  return (
    <div className={styles.topnav}>
      <div className={styles.logo}>
        <Logo size={26} />
        <span className={styles.logoText}>Infinity</span>
      </div>
      <div className={styles.divider} />
      <button
        className={cn(styles.tab, activeTab === 'play' && styles.active)}
        onClick={() => setTab('play')}
      >
        <Icons.Play /> Играть
      </button>
      <button
        className={cn(styles.tab, activeTab === 'info' && styles.active)}
        onClick={() => setTab('info')}
      >
        <Icons.Info /> О программе
      </button>
      <div className={styles.right}>
        <div className={styles.user} onClick={onLogout} title="Нажмите для выхода">
          <div className={styles.avatar}>{user.username[0].toUpperCase()}</div>
          <span className={styles.username}>{user.username}</span>
        </div>
      </div>
    </div>
  )
}
