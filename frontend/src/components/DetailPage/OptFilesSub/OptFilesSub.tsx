import { Toggle } from '../../ui/Toggle/Toggle'
import type { OptFile } from '../../../types'
import styles from './OptFilesSub.module.css'

interface OptFilesSubProps {
  files: OptFile[]
  onToggle: (id: string) => void
}

export function OptFilesSub({ files, onToggle }: OptFilesSubProps) {
  const activeCount = files.filter(f => f.on).length

  return (
    <div className={styles.sub}>
      <div className={styles.header}>
        Моды и ресурспаки — {activeCount} из {files.length} активно
      </div>
      <div className={styles.list}>
        {files.map(f => (
          <div
            key={f.id}
            className={styles.item}
            onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = 'rgba(255,255,255,0.14)' }}
            onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--glass-border)' }}
          >
            <div className={styles.info}>
              <div className={styles.name}>{f.name}</div>
              <div className={styles.meta}>{f.desc} · {f.size}</div>
            </div>
            <Toggle checked={f.on} onChange={() => onToggle(f.id)} />
          </div>
        ))}
      </div>
    </div>
  )
}
