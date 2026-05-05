import type { Profile, OptFile } from '../types'

export const PROFILES: Profile[] = [
  {
    id: 1,
    slug: 'survival',
    title: 'Survival',
    version: '1.20.4',
    desc: 'Классическое выживание с экономикой, приватами и дружелюбным комьюнити. Стройте города, торгуйте, исследуйте.',
    players: { online: 47, max: 200 },
    status: 'online',
    bg: 'linear-gradient(140deg,#071a07 0%,#0c2e14 50%,#071510 100%)',
    accent: '#4ade80',
    accentDim: 'rgba(74,222,128,0.15)',
    icon: '🌲',
  },
  {
    id: 2,
    slug: 'skyblock',
    title: 'SkyBlock',
    version: '1.20.1',
    desc: 'Начните с крошечного острова в небе и постройте цивилизацию. Уникальные квесты и еженедельные ивенты.',
    players: { online: 123, max: 500 },
    status: 'online',
    bg: 'linear-gradient(140deg,#060d1f 0%,#0d1f40 50%,#060e22 100%)',
    accent: '#60a5fa',
    accentDim: 'rgba(96,165,250,0.15)',
    icon: '🏝️',
  },
  {
    id: 3,
    slug: 'minigames',
    title: 'MiniGames',
    version: '1.20.4',
    desc: 'Более 20 мини-игр: BedWars, SkyWars, Murder Mystery, Spleef. Зарабатывайте монеты и взбирайтесь в рейтинге.',
    players: { online: 0, max: 300 },
    status: 'offline',
    bg: 'linear-gradient(140deg,#160820 0%,#250d38 50%,#110618 100%)',
    accent: '#c084fc',
    accentDim: 'rgba(192,132,252,0.15)',
    icon: '🎮',
  },
]

export const OPT_FILES: OptFile[] = [
  { id: 'optifine', name: 'OptiFine HD', desc: 'Оптимизация графики и FPS', size: '2.4 MB', on: true },
  { id: 'shaders', name: 'Complementary Shaders', desc: 'Фотореалистичные шейдеры', size: '1.1 MB', on: false },
  { id: 'rp', name: 'Infinity Resource Pack', desc: 'Официальный ресурспак сервера', size: '18.7 MB', on: true },
  { id: 'minimap', name: "Xaero's Minimap", desc: 'Карта мира прямо в игре', size: '0.8 MB', on: false },
]
