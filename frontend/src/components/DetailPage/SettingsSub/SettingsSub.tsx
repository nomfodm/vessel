import { Button } from '../../ui/Button/Button'
import { Icons } from '../../Icons/Icons'
import styles from './SettingsSub.module.css'

const TOTAL_RAM = 16384

interface SettingsSubProps {
  ram: number
  path: string
  onRamChange: (ram: number) => void
}

export function SettingsSub({ ram, path, onRamChange }: SettingsSubProps) {
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
              <span className={styles.ramTotal}>/ {TOTAL_RAM} МБ</span>
            </>
          )}
        </div>
        <div className={styles.sliderWrap}>
          <input
            type="range"
            min={1024}
            max={TOTAL_RAM}
            step={256}
            value={ram}
            onChange={e => onRamChange(Number(e.target.value))}
          />
          <div className={styles.sliderLabels}>
            <span>АВТО</span>
            <span>{TOTAL_RAM} МБ</span>
          </div>
        </div>
      </div>

      <div className={styles.section}>
        <div className={styles.label}>Папка с файлами игры</div>
        <div className={styles.pathDisplay}>{path}</div>
        <div className={styles.pathActions}>
          <Button variant="ghost" style={{ height: 36, fontSize: 12 }}>
            <Icons.Folder /> Открыть
          </Button>
          <Button variant="ghost" style={{ height: 36, fontSize: 12 }}>
            Изменить
          </Button>
        </div>
      </div>
    </div>
  )
}
