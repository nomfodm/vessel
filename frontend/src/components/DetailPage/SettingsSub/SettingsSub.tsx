import { Button } from '../../ui/Button/Button'
import { Icons } from '../../Icons/Icons'
import styles from './SettingsSub.module.css'

interface SettingsSubProps {
  ram: number
  path: string
  total: number
  onRamChange: (ram: number) => void
  onOpen: () => void
}

export function SettingsSub({ ram, path, total, onRamChange, onOpen }: SettingsSubProps) {
  return (
    <div className={styles.sub}>
      <div className={styles.section}>
        <div className={styles.label}>Оперативная память</div>
        <div className={styles.ramValue}>
          {ram <= 1024 ? (
            <span className={styles.ramAuto}>АВТО</span>
          ) : (
            <>
              <span className={styles.ramNum}>{ram}</span>{' '}
              <span className={styles.ramTotal}>/ {total} МБ</span>
            </>
          )}
        </div>
        <div className={styles.sliderWrap}>
          <input
            type="range"
            min={1024}
            max={total}
            step={256}
            value={Math.min(ram, total)}
            onChange={e => onRamChange(Number(e.target.value))}
          />
          <div className={styles.sliderLabels}>
            <span>АВТО</span>
            <span>{total} МБ</span>
          </div>
        </div>
      </div>

      <div className={styles.section}>
        <div className={styles.label}>Папка профиля</div>
        <div className={styles.pathDisplay}>{path || '—'}</div>
        <div className={styles.pathActions}>
          <Button variant="ghost" style={{ height: 36, fontSize: 12 }} onClick={onOpen}>
            <Icons.Folder /> Открыть
          </Button>
        </div>
      </div>
    </div>
  )
}
