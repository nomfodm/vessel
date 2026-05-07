import type { ReactNode } from 'react'
import type { BadgeVariant } from '../../../types'
import styles from './Badge.module.css'

interface BadgeProps {
  variant: BadgeVariant
  children?: ReactNode
}

export function Badge({ variant, children }: BadgeProps) {
  const hasDot = variant === 'online' || variant === 'pinging'
  return (
    <span className={`${styles.badge} ${styles[variant]}`}>
      {hasDot && <span className={styles.dotPulse} data-anim-infinite />}
      {children}
    </span>
  )
}
