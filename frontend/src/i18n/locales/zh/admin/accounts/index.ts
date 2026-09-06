import list from './list'
import platform from './platform'
import form from './form'
import vendor from './vendor'
import model from './model'
import oauth from './oauth'

export default {
  accounts: {
    ...list,
    ...platform,
    ...form,
    ...vendor,
    ...model,
    ...oauth,
  },
}
