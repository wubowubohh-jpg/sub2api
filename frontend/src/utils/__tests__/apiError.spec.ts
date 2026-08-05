import { describe, expect, it } from 'vitest'

import { extractApiErrorMessage } from '../apiError'

describe('extractApiErrorMessage', () => {
  it('prefers the backend error over a generic interceptor message', () => {
    expect(
      extractApiErrorMessage({
        status: 400,
        message: 'Request failed with status code 400',
        error: '账单汇总查询失败',
      }),
    ).toBe('账单汇总查询失败')
  })

  it('uses the interceptor message when the backend error is absent', () => {
    expect(
      extractApiErrorMessage({
        status: 502,
        message: 'Request failed with status code 502',
      }),
    ).toBe('Request failed with status code 502')
  })

  it('keeps a descriptive backend message ahead of an error code', () => {
    expect(
      extractApiErrorMessage({
        status: 400,
        message: '请求参数格式错误',
        error: 'INVALID_REQUEST',
      }),
    ).toBe('请求参数格式错误')
  })
})
