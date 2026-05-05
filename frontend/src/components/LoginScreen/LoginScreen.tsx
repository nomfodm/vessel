import { useState } from 'react'
import type { FormEvent } from 'react'
import { Logo } from '../Logo/Logo'
import { Button } from '../ui/Button/Button'
import { Input } from '../ui/Input/Input'
import { Spinner } from '../ui/Spinner/Spinner'
import { Icons } from '../Icons/Icons'
import type { User } from '../../types'
import styles from './LoginScreen.module.css'

interface LoginScreenProps {
  onLogin: (user: User) => void
}

export function LoginScreen({ onLogin }: LoginScreenProps) {
  const [form, setForm] = useState({ user: '', pass: '' })
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setErr('')
    if (!form.user.trim() || !form.pass.trim()) { setErr('Заполните все поля'); return }
    if (form.user.length < 3) { setErr('Имя пользователя слишком короткое'); return }
    if (form.pass.length < 6) { setErr('Пароль: минимум 6 символов'); return }
    setLoading(true)
    await new Promise(r => setTimeout(r, 900))
    setLoading(false)
    onLogin({ username: form.user })
  }

  return (
    <div className={styles.screen}>
      <div className={styles.glow} />
      <div className={styles.panel}>
        <div className={styles.header}>
          <div className={styles.logoFloat}>
            <Logo size={52} />
          </div>
          <div>
            <div className={styles.title}>
              <span className={styles.grad}>Infinity Launcher</span>
            </div>
            <div className={styles.subtitle}>Войдите, чтобы начать играть</div>
          </div>
        </div>

        <div className={styles.card}>
          <form onSubmit={submit} className={styles.form}>
            <Input
              label="Имя пользователя"
              placeholder="thebestplayer"
              autoFocus
              value={form.user}
              onChange={e => setForm({ ...form, user: e.target.value })}
            />
            <Input
              label="Пароль"
              type="password"
              placeholder="••••••••"
              value={form.pass}
              onChange={e => setForm({ ...form, pass: e.target.value })}
            />
            {err && (
              <div className={styles.error}>
                <Icons.Err color="#fca5a5" /> {err}
              </div>
            )}
            <Button
              variant="primary"
              type="submit"
              disabled={loading}
              className={styles.submitBtn}
              style={{ height: 46, fontSize: 15, borderRadius: 12 }}
            >
              {loading ? <><Spinner /> Входим...</> : 'Войти'}
            </Button>
          </form>

          <div className={styles.footer}>
            <span className={styles.footerText}>
              Нет аккаунта?{' '}
              <span className={styles.registerLink} onClick={() => alert('Откроется сайт для регистрации')}>
                Создайте же его! ↗
              </span>
            </span>
          </div>
        </div>

        <div className={styles.version}>Infinity Launcher v2.1.0 · Все права защищены</div>
      </div>
    </div>
  )
}
