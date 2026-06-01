import { useState } from 'react'
import type { FormEvent } from 'react'
import { Logo } from '../Logo/Logo'
import { Button } from '../ui/Button/Button'
import { Input } from '../ui/Input/Input'
import { Spinner } from '../ui/Spinner/Spinner'
import { Icons } from '../Icons/Icons'
import { cn } from '../../utils/cn'
import type { User } from '../../types'
import styles from './LoginScreen.module.css'
import { Browser } from "@wailsio/runtime"
import { Service as AuthService } from '../../../bindings/github.com/nomfodm/vessel/internal/auth'

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
    try {
      await AuthService.Login(form.user, form.pass)
      const u = await AuthService.CurrentUser()
      onLogin({ username: u.username })
    } catch {
      setErr('Неверный логин или пароль')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.screen}>
      <div className={styles.glow} />
      <div className={styles.panel}>
        <div className={styles.header}>
          <div className={styles.logoFloat} data-anim-infinite>
            <Logo size={48} />
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
            <div className={styles.bottomGroup}>
              <div className={cn(styles.errorWrapper, !!err && styles.errorVisible)}>
                <div className={styles.errorInner}>
                  <div className={styles.error}>
                    <Icons.Err color="#fca5a5" /> {err}
                  </div>
                </div>
              </div>
              <Button
                variant="primary"
                type="submit"
                disabled={loading}
                className={styles.submitBtn}
                style={{ height: 44, fontSize: 15, borderRadius: 12 }}
              >
                {loading ? <><Spinner /> Входим...</> : 'Войти'}
              </Button>
            </div>
          </form>

          <div className={styles.footer}>
            <span className={styles.footerText}>
              Нет аккаунта?{' '}
              <span className={styles.registerLink} onClick={() => Browser.OpenURL("https://infinityserver.ru/register")}>
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
