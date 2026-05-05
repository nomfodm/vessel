import styles from './ProgressBar.module.css'

interface ProgressBarProps {
  value: number
}

export function ProgressBar({ value }: ProgressBarProps) {
  return (
    <div className={styles.track}>
      <div className={styles.fill} style={{ width: `${value}%` }} />
    </div>
  )
}
