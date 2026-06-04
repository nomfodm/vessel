import { NeonFlash } from '../NeonFlash/NeonFlash'
import styles from './ErrorScreen.module.css'

interface ServerStatus {
  name: string
  ok: boolean
  state: string
}

type Reason = 'offline' | 'maintenance' | 'off'

interface ErrorScreenProps {
  statuses: ServerStatus[]
  reason: Reason
  retrying: boolean
  onRetry: () => void
}

const COPY: Record<Reason, { mark: string; markClass: string; title: string; subtitle: string }> = {
  offline: {
    mark: '!',
    markClass: 'markDown',
    title: 'Нет соединения с серверами',
    subtitle: 'Не удалось связаться с серверами Infinity. Проверьте интернет-соединение и попробуйте снова.',
  },
  maintenance: {
    mark: '⚙',
    markClass: 'markWarn',
    title: 'Технические работы',
    subtitle: 'На серверах Infinity идут технические работы. Зайдите немного позже.',
  },
  off: {
    mark: '!',
    markClass: 'markDown',
    title: 'Серверы недоступны',
    subtitle: 'Серверы Infinity временно отключены. Попробуйте зайти позже.',
  },
}

function rowLabel(s: ServerStatus): { text: string; cls: string } {
  switch (s.state) {
    case 'operational':
      return { text: 'доступен', cls: styles.up }
    case 'maintenance':
      return { text: 'техработы', cls: styles.warn }
    case 'off':
      return { text: 'отключён', cls: styles.down }
    default:
      return { text: 'недоступен', cls: styles.down }
  }
}

export function ErrorScreen({ statuses, reason, retrying, onRetry }: ErrorScreenProps) {
  const copy = COPY[reason]
  return (
    <div className={styles.screen}>
      <NeonFlash />
      <div className={styles.panel}>
        <div className={`${styles.mark} ${styles[copy.markClass]}`}>{copy.mark}</div>
        <div className={styles.title}>{copy.title}</div>
        <div className={styles.subtitle}>{copy.subtitle}</div>

        {statuses.length > 0 && (
          <div className={styles.list}>
            {statuses.map(s => {
              const l = rowLabel(s)
              return (
                <div key={s.name} className={styles.row}>
                  <span className={styles.rowName}>{s.name}</span>
                  <span className={l.cls}>{l.text}</span>
                </div>
              )
            })}
          </div>
        )}

        <button className={styles.retry} onClick={onRetry} disabled={retrying}>
          {retrying ? 'Проверка…' : 'Повторить'}
        </button>
      </div>
    </div>
  )
}
