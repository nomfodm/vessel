import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { NeonFlash } from '../NeonFlash/NeonFlash'
import { ProgressBar } from '../ui/ProgressBar/ProgressBar'
import { Spinner } from '../ui/Spinner/Spinner'
import { Service as UpdaterService } from '../../../bindings/github.com/nomfodm/vessel/internal/updater'
import styles from './UpdateScreen.module.css'

interface UpdateScreenProps {
  latest: string
  changelog: string[]
}

type State = 'idle' | 'updating' | 'restart' | 'error'

export function UpdateScreen({ latest, changelog }: UpdateScreenProps) {
  const [state, setState] = useState<State>('idle')
  const [progress, setProgress] = useState(0)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    const off = Events.On('update:progress', (e: { data: { pct: number } }) => setProgress(e.data.pct))
    return () => { off() }
  }, [])

  function install() {
    setState('updating')
    setProgress(0)
    setErrorMsg('')
    UpdaterService.Apply()
      .then(() => {
        setState('restart')
        return UpdaterService.Restart()
      })
      .catch((e: unknown) => {
        setState('error')
        setErrorMsg(String(e ?? 'Не удалось обновить лаунчер'))
      })
  }

  return (
    <div className={styles.screen}>
      <NeonFlash />
      <div className={styles.panel}>
        <div className={styles.badge}>↑</div>
        <div className={styles.title}>Обязательное обновление</div>
        <div className={styles.subtitle}>
          Для продолжения необходимо обновить лаунчер до версии{' '}
          <span className={styles.version}>{latest}</span>
        </div>

        {changelog.length > 0 && (
          <ul className={styles.changelog}>
            {changelog.map((c, i) => <li key={i}>{c}</li>)}
          </ul>
        )}

        {state === 'idle' && (
          <button className={styles.btn} onClick={install}>
            Установить обновление
          </button>
        )}

        {state === 'updating' && (
          <div className={styles.progress}>
            <div className={styles.progressLabel}>
              <Spinner /> Загрузка... {progress}%
            </div>
            <ProgressBar value={progress} />
          </div>
        )}

        {state === 'restart' && (
          <div className={styles.progressLabel}>
            <Spinner /> Перезапуск...
          </div>
        )}

        {state === 'error' && (
          <>
            <div className={styles.error}>{errorMsg}</div>
            <button className={styles.btn} onClick={install}>
              Попробовать снова
            </button>
          </>
        )}
      </div>
    </div>
  )
}
