import { Logo } from '../Logo/Logo'
import { cn } from '../../utils/cn'
import styles from './BootScreen.module.css'

interface BootScreenProps {
  message: string
  closing: boolean
}

export function BootScreen({ message, closing }: BootScreenProps) {
  return (
    <div className={cn(styles.screen, closing && styles.closing)}>
      <div className={styles.logoWrap} data-anim-infinite>
        <Logo size={56} />
      </div>
      <div className={styles.spinnerRow}>
        <div className={styles.spinner} />
        <span className={styles.message}>{message}</span>
      </div>
    </div>
  )
}
