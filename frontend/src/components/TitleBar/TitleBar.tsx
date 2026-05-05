import { Window } from '@wailsio/runtime'
import styles from './TitleBar.module.css'

export function TitleBar() {
  return (
    <div className={styles.titlebar}>
      <span className={styles.title}>Infinity Launcher</span>
      <div className={styles.controls}>
        <button className={`${styles.ctrl} ${styles.minimize}`} onClick={() => Window.Minimise()}>
          &#8722;
        </button>
        <button className={`${styles.ctrl} ${styles.close}`} onClick={() => Window.Close()}>
          &#10005;
        </button>
      </div>
    </div>
  )
}
