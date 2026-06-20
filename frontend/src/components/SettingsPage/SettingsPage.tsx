import { useState, useEffect } from 'react'
import { Button } from '../ui/Button/Button'
import { Icons } from '../Icons/Icons'
import { Service as SystemService } from '../../../bindings/github.com/nomfodm/vessel/internal/system'
import styles from './SettingsPage.module.css'

interface SettingsPageProps {
  onRelocatingChange: (v: boolean) => void
}

export function SettingsPage({ onRelocatingChange }: SettingsPageProps) {
  const [dataRoot, setDataRoot] = useState('')
  const [changing, setChanging] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    SystemService.CurrentDataRoot().then(setDataRoot).catch(() => { /* ignore */ })
  }, [])

  function open() {
    SystemService.OpenDataRoot().catch(() => { /* ignore */ })
  }

  // Pick a new folder, move everything there, then restart so the launcher picks
  // up the new layout. A cancelled picker returns '' — a no-op.
  async function change() {
    setErr('')
    let dir = ''
    try {
      dir = await SystemService.PickDataRoot()
    } catch {
      return
    }
    if (!dir) return
    setChanging(true)
    onRelocatingChange(true)
    try {
      await SystemService.ChangeDataRoot(dir)
      await SystemService.Restart()
    } catch (e) {
      setChanging(false)
      onRelocatingChange(false)
      setErr(typeof e === 'string' ? e : (e as Error)?.message || 'Не удалось перенести папку')
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.sectionLabel}>Настройки лаунчера</div>

      <div className={styles.section}>
        <div className={styles.label}>Папка с файлами игры</div>
        <div className={styles.desc}>
          Здесь хранятся клиенты, библиотеки и Java. При переносе на тот же диск файлы
          перемещаются мгновенно; на другой — копируются заново, после чего лаунчер перезапустится.
        </div>
        <div className={styles.pathDisplay}>{dataRoot || '…'}</div>
        <div className={styles.pathActions}>
          <Button variant="ghost" style={{ height: 36, fontSize: 12 }} onClick={open} disabled={changing}>
            <Icons.Folder /> Открыть
          </Button>
          <Button variant="ghost" style={{ height: 36, fontSize: 12 }} onClick={change} disabled={changing}>
            {changing ? 'Перенос…' : 'Изменить'}
          </Button>
        </div>
        {err && <div className={styles.error}>{err}</div>}
      </div>
    </div>
  )
}
