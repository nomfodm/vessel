import type { InputHTMLAttributes } from 'react'
import styles from './Input.module.css'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
}

export function Input({ label, ...props }: InputProps) {
  return (
    <div className={styles.wrap}>
      {label && <label className={styles.label}>{label}</label>}
      <input className={styles.field} {...props} />
    </div>
  )
}
