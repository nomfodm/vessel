import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../../utils/cn'
import type { ButtonVariant } from '../../../types'
import styles from './Button.module.css'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant: ButtonVariant
}

export function Button({ variant, className, children, ...props }: ButtonProps) {
  const variantClass: Record<ButtonVariant, string> = {
    primary: styles.primary,
    danger: styles.danger,
    ghost: styles.ghost,
    'icon-sq': styles.iconSq,
  }

  return (
    <button className={cn(styles.btn, variantClass[variant], className)} {...props}>
      {children}
    </button>
  )
}
