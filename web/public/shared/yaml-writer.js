/**
 * YAML Writer - 轻量级 YAML 序列化器
 * 共享模块：WebUI (ES import） 均可使用
 *
 * 用于生成 Centag 配置文件
 */
class YamlWriter {
  /**
   * 将对象序列化为 YAML 字符串
   * @param {*} obj
   * @param {number} [indent=0]
   * @returns {string}
   */
  static stringify(obj, indent = 0) {
    if (obj === null || obj === undefined) return 'null'
    if (typeof obj === 'boolean') return obj.toString()
    if (typeof obj === 'number') return obj.toString()
    if (typeof obj === 'string') return this.escapeString(obj)
    if (Array.isArray(obj)) return this.stringifyArray(obj, indent)
    if (typeof obj === 'object') return this.stringifyObject(obj, indent)
    return String(obj)
  }

  /**
   * 转义字符串
   * @param {string} str
   * @returns {string}
   */
  static escapeString(str) {
    if (str === '') return '""'
    if (str.includes(':') || str.includes('#') || str.includes('\n') ||
        str.startsWith(' ') || str.endsWith(' ') ||
        str === 'true' || str === 'false' || str === 'null' ||
        /^[\d.e+-]+$/i.test(str)) {
      return '"' + str.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
    }
    return str
  }

  /**
   * 序列化数组
   * @param {Array} arr
   * @param {number} indent
   * @returns {string}
   */
  static stringifyArray(arr, indent) {
    if (arr.length === 0) return '[]'
    const prefix = '  '.repeat(indent)
    const lines = arr.map(item => {
      if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
        const inner = this.stringifyObject(item, indent + 1)
        const parts = inner.split('\n')
        const firstLine = parts[0].trimStart()
        const rest = parts.slice(1).join('\n')
        return `${prefix}- ${firstLine}\n${rest}`
      }
      return `${prefix}- ${this.stringify(item, indent)}`
    })
    return lines.join('\n')
  }

  /**
   * 序列化对象
   * @param {Object} obj
   * @param {number} indent
   * @returns {string}
   */
  static stringifyObject(obj, indent) {
    const keys = Object.keys(obj)
    if (keys.length === 0) return '{}'
    const prefix = '  '.repeat(indent)
    const lines = keys.map(key => {
      const value = obj[key]
      const keyStr = this.escapeKey(key)
      if (value === null || value === undefined) {
        return `${prefix}${keyStr}: null`
      }
      if (typeof value === 'object' && !Array.isArray(value)) {
        const inner = this.stringifyObject(value, indent + 1)
        return `${prefix}${keyStr}:\n${inner}`
      }
      if (Array.isArray(value)) {
        if (value.length === 0) {
          return `${prefix}${keyStr}: []`
        }
        const inner = this.stringifyArray(value, indent + 1)
        return `${prefix}${keyStr}:\n${inner}`
      }
      return `${prefix}${keyStr}: ${this.stringify(value, indent)}`
    })
    return lines.join('\n')
  }

  /**
   * 转义键名
   * @param {string} key
   * @returns {string}
   */
  static escapeKey(key) {
    if (/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(key)) {
      return key
    }
    return '"' + key.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
  }
}

// UMD: 兼容 script 标签 (WebUI) 和 ES module (webui)
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { YamlWriter }
}

if (typeof globalThis !== 'undefined') {
  globalThis.YamlWriter = YamlWriter
}
