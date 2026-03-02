import { useMemo } from 'react'
import { createAvatar } from '@dicebear/core'
import { avataaars } from '@dicebear/collection'
import { shapes } from '@dicebear/collection'

interface AvatarProps {
  seed: string
  type?: 'user' | 'thread'
  size?: number
  className?: string
}

export default function Avatar({ seed, type = 'user', size = 40, className = '' }: AvatarProps) {
  const svg = useMemo(() => {
    const avatar = createAvatar(type === 'user' ? avataaars : shapes, {
      seed,
      size,
    })
    return avatar.toDataUri()
  }, [seed, type, size])

  return (
    <img
      src={svg}
      alt=""
      width={size}
      height={size}
      className={`rounded-full ${className}`}
    />
  )
}
