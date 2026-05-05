import styles from './Toggle.module.css'

interface ToggleProps {
  checked: boolean
  onChange: (checked: boolean) => void
}

export function Toggle({ checked, onChange }: ToggleProps) {
  return (
    <button
      type="button"
      className={`${styles.toggle} ${checked ? styles.on : styles.off}`}
      onClick={() => onChange(!checked)}
    >
      <div className={styles.knob} />
    </button>
  )
}
