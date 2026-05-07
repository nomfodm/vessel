import styles from './Spinner.module.css'

export function Spinner() {
  return <div className={styles.spinner} style={{ animation: 'spin 0.7s linear infinite' }} />
}
