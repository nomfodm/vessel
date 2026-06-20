import { useState, useEffect } from 'react'
import type { ReactNode } from 'react'
import { Events } from '@wailsio/runtime'
import { Logo } from '../Logo/Logo'
import { Spinner } from '../ui/Spinner/Spinner'
import { ProgressBar } from '../ui/ProgressBar/ProgressBar'
import { Icons } from '../Icons/Icons'
import { Service as UpdaterService } from '../../../bindings/github.com/nomfodm/vessel/internal/updater'
import styles from './InfoPage.module.css'

type CheckState = 'check' | 'none' | 'found' | 'updating' | 'restart' | 'error'

export function InfoPage() {
  const [state, setState] = useState<CheckState>('check')
  const [version, setVersion] = useState('')
  const [latest, setLatest] = useState('')
  const [changelog, setChangelog] = useState<string[]>([])
  const [mandatory, setMandatory] = useState(false)
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    let cancelled = false
    UpdaterService.Version().then(v => { if (!cancelled) setVersion(v) }).catch(() => {})
    UpdaterService.Check()
      .then(info => {
        if (cancelled) return
        setLatest(info.latest)
        setChangelog(info.changelog ?? [])
        setMandatory(info.isMandatory)
        setState(info.needsUpdate ? 'found' : 'none')
      })
      .catch(() => { if (!cancelled) setState('error') })
    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    const off = Events.On('update:progress', (e: { data: { pct: number } }) => setProgress(e.data.pct))
    return () => { off() }
  }, [])

  function install() {
    setState('updating')
    setProgress(0)
    UpdaterService.Apply()
      .then(() => {
        setState('restart')
        return UpdaterService.Restart() // process exits; the new binary takes over
      })
      .catch(() => setState('error'))
  }

  const statusUI: Record<CheckState, ReactNode> = {
    check: (
      <div className={styles.statusRow}>
        <Spinner /><span className={styles.statusText}>Проверка обновлений...</span>
      </div>
    ),
    none: (
      <div className={styles.statusRow}>
        <Icons.Check /><span className={`${styles.statusText} ${styles.green}`}>Установлена последняя версия</span>
      </div>
    ),
    found: (
      <div className={styles.statusCol}>
        <div className={styles.statusRow}>
          <Icons.Warn />
          <span className={`${styles.statusText} ${styles.yellow}`}>
            Доступна версия {latest}{mandatory && ' · обязательное'}
          </span>
        </div>
        {changelog.length > 0 && (
          <ul className={styles.changelog}>
            {changelog.map((c, i) => <li key={i}>{c}</li>)}
          </ul>
        )}
        <button className={styles.updateLink} onClick={install}>Установить обновление</button>
      </div>
    ),
    updating: (
      <div className={styles.statusCol}>
        <div className={styles.statusRow}>
          <Spinner /><span className={styles.statusText}>Загрузка обновления... {progress}%</span>
        </div>
        <div className={styles.progressWrap}>
          <ProgressBar value={progress} />
        </div>
      </div>
    ),
    restart: (
      <div className={styles.statusRow}>
        <Spinner /><span className={styles.statusText}>Перезапуск...</span>
      </div>
    ),
    error: (
      <div className={styles.statusRow}>
        <Icons.Err /><span className={`${styles.statusText} ${styles.red}`}>Не удалось обновить</span>
      </div>
    ),
  }

  return (
    <div className={styles.page}>
      <div className={styles.sectionLabel}>О программе</div>
      <div className={styles.content}>
        <div className={styles.left}>
          <div className={styles.titleRow}>
            <span className={styles.appTitle}>
              Infinity <span className={styles.grad}>Launcher</span>
            </span>
            <span className={styles.version}>{version || '…'}</span>
          </div>

          {statusUI[state]}
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
