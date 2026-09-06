import general from './general'
import platform from './platform'
import site from './site'
import email from './email'
import advanced from './advanced'

export default {
  settings: {
    ...general,
    ...platform,
    ...site,
    ...email,
  },
  ...advanced,
}
