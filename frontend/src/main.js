import './style.css'
import App from './App.svelte'
import { exposePluginAPI, initImageFallback } from './pluginRegistry.js'

exposePluginAPI()
initImageFallback()

const target = document.getElementById('app')
if (!target) {
  throw new Error('missing #app root')
}
const app = new App({ target })

export default app
