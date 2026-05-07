import { useState, useEffect } from 'react'
import type { ReactNode } from 'react'
import { Logo } from '../Logo/Logo'
import { Spinner } from '../ui/Spinner/Spinner'
import { Icons } from '../Icons/Icons'
import type { UpdateState } from '../../types'
import styles from './InfoPage.module.css'

const ALL_STATES: UpdateState[] = ['check', 'none', 'found', 'updating', 'restart', 'error']

export function InfoPage() {
  const [us, setUs] = useState<UpdateState>('check')
  const VERSION = 'v2.1.0'

  useEffect(() => {
    const t = setTimeout(() => setUs('found'), 1300)
    return () => clearTimeout(t)
  }, [])

  function doUpdate() {
    setUs('updating')
    setTimeout(() => {
      setUs('restart')
      setTimeout(() => setUs('none'), 1500)
    }, 2200)
  }

  function cycle() {
    setUs(prev => ALL_STATES[(ALL_STATES.indexOf(prev) + 1) % ALL_STATES.length])
  }

  const statusUI: Record<UpdateState, ReactNode> = {
    check: (
      <div className={styles.statusRow}>
        <Spinner /><span className={styles.statusText}>Проверка обновлений...</span>
      </div>
    ),
    none: (
      <div className={styles.statusRow}>
        <Icons.Check /><span className={`${styles.statusText} ${styles.green}`}>Обновлений не найдено</span>
      </div>
    ),
    found: (
      <div className={styles.statusCol}>
        <div className={styles.statusRow}>
          <Icons.Warn /><span className={`${styles.statusText} ${styles.yellow}`}>Найдены необязательные обновления</span>
        </div>
        <button className={styles.updateLink} onClick={doUpdate}>Обновиться</button>
      </div>
    ),
    updating: (
      <div className={styles.statusRow}>
        <Spinner /><span className={styles.statusText}>Обновление...</span>
      </div>
    ),
    restart: (
      <div className={styles.statusRow}>
        <Spinner /><span className={styles.statusText}>Перезапуск...</span>
      </div>
    ),
    error: (
      <div className={styles.statusRow}>
        <Icons.Err /><span className={`${styles.statusText} ${styles.red}`}>Ошибка при обновлении</span>
      </div>
    ),
  }

  const techInfo: [string, string][] = [
    ['Движок', 'Wails v3 + React 18'],
    ['Платформы', 'Windows · macOS · Linux'],
    ['Протокол', 'Minecraft Auth v3'],
  ]

  return (
    <div className={styles.page}>
      <div className={styles.sectionLabel}>О программе</div>
      <div className={styles.content}>
        <div className={styles.left}>
          <div className={styles.titleRow}>
            <span className={styles.appTitle}>
              Infinity <span className={styles.grad}>Launcher</span>
            </span>
            <span className={styles.version}>{VERSION}</span>
          </div>

          {statusUI[us]}

          <div className={styles.techTable}>
            {techInfo.map(([k, v]) => (
              <div key={k} className={styles.techRow}>
                <span className={styles.techKey}>{k}</span>
                <span className={styles.techVal}>{v}</span>
              </div>
            ))}
          </div>

          <button className={styles.demoBtn} onClick={cycle}>[демо] следующий статус →</button>
        </div>

        <div className={styles.right}>
          <div className={styles.floatLogo} data-anim-infinite>
            <Logo size={96} />
          </div>
        </div>
      </div>
    </div>
  )
}
