import assert from 'node:assert/strict'
import { beforeEach, describe, test } from 'node:test'

class LocalStorageMock {
  private store = new Map<string, string>()

  clear() {
    this.store.clear()
  }

  getItem(key: string) {
    return this.store.get(key) ?? null
  }

  removeItem(key: string) {
    this.store.delete(key)
  }

  setItem(key: string, value: string) {
    this.store.set(key, value)
  }
}

const localStorage = new LocalStorageMock()

Object.defineProperty(globalThis, 'window', {
  value: { localStorage },
  configurable: true,
})

const { useAuthStore } = await import('./auth-store')

describe('auth-store', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.getState().auth.reset()
  })

  test('setUser persists both user payload and uid header source', () => {
    useAuthStore.getState().auth.setUser({
      id: 42,
      username: 'qianshan',
      role: 100,
    })

    assert.equal(localStorage.getItem('uid'), '42')

    const savedUser = localStorage.getItem('user')
    assert.notEqual(savedUser, null)
    assert.equal(JSON.parse(savedUser as string).username, 'qianshan')
  })

  test('reset clears both user payload and uid header source', () => {
    useAuthStore.getState().auth.setUser({
      id: 42,
      username: 'qianshan',
      role: 100,
    })

    useAuthStore.getState().auth.reset()

    assert.equal(localStorage.getItem('user'), null)
    assert.equal(localStorage.getItem('uid'), null)
  })
})
