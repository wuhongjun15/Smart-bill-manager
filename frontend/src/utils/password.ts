/**
 * Password strength utility functions
 */

export interface PasswordStrength {
  level: 'weak' | 'medium' | 'strong'
  text: string
}

/**
 * Check password strength
 * @param password - The password to check
 * @returns Password strength information
 */
export function checkPasswordStrength(password: string): PasswordStrength {
  if (password.length < 6) {
    return { level: 'weak', text: '弱' }
  } else if (password.length < 10) {
    return { level: 'medium', text: '中等' }
  } else if (password.length >= 10 && /[A-Z]/.test(password) && /[0-9]/.test(password) && /[^A-Za-z0-9]/.test(password)) {
    return { level: 'strong', text: '强' }
  } else {
    return { level: 'medium', text: '中等' }
  }
}
