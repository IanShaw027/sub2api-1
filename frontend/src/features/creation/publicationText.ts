import { useI18n } from 'vue-i18n'

const en = {
  wall: 'Inspiration', all: 'All', images: 'Images', videos: 'Videos', search: 'Search titles and prompts',
  featured: 'Featured inspiration', community: 'Community works',
  refresh: 'Refresh', empty: 'No public works yet', noResults: 'No matching works', loading: 'Loading...',
  loadFailed: 'Could not load public works', retry: 'Retry', more: 'Load more', preview: 'Preview',
  unavailable: 'Media unavailable', create: 'Create with this prompt', prompt: 'Prompt', model: 'Model',
  published: 'Published publicly', close: 'Close', cancel: 'Cancel', title: 'Title',
  publish: 'Publish publicly', publishing: 'Publishing...', publishFailed: 'Publication failed. Retry with the same details.',
  publicNotice: 'Publishing uploads this file to the server. Anyone with the public media link can view it; its title, prompt and model appear in the inspiration gallery.',
  consent: 'I confirm this work and its prompt may be viewed publicly.',
  withdraw: 'Withdraw publication', withdrawNotice: 'Remove this work from the public gallery and disable its media link? Your local copy is kept. Copies already downloaded by others cannot be recalled.',
  withdrawFailed: 'Could not withdraw this publication', withdrawing: 'Withdrawing...',
  fileInvalid: 'Choose a PNG, JPEG, GIF, WebP, MP4 or WebM file up to 64 MiB.',
  required: 'Enter a title and confirm public visibility.', publicLink: 'Open public media',
  checking: 'Checking publication status...', unknown: 'Publication status could not be confirmed. Retry to check or complete the same publication.',
  mine: 'Your publication',
  withdrawn: 'This publication has been withdrawn.',
  pending: 'Publication is incomplete and is not publicly visible.',
  resume: 'Complete public publication',
  cleanup: 'Cancel pending publication',
  cleanupNotice: 'Cancel this pending publication and remove its uploaded server file? Your local work is kept.',
  cleanupFailed: 'Server cleanup failed. You can retry cancellation.',
  pendingWithdrawn: 'Publication was cancelled. Server cleanup can be retried.',
}
const zh: Record<keyof typeof en, string> = {
  wall: '灵感墙', all: '全部', images: '图片', videos: '视频', search: '搜索标题与提示词',
  featured: '精选灵感', community: '社区作品',
  refresh: '刷新', empty: '暂无公开作品', noResults: '没有匹配的作品', loading: '加载中...',
  loadFailed: '公开作品加载失败', retry: '重试', more: '加载更多', preview: '预览',
  unavailable: '媒体暂不可用', create: '一键创作', prompt: '提示词', model: '模型',
  published: '已公开发布', close: '关闭', cancel: '取消', title: '标题',
  publish: '公开发布', publishing: '发布中...', publishFailed: '发布失败，可使用相同内容重试。',
  publicNotice: '发布会将此文件上传到服务器。任何持有公开媒体链接的人均可查看；标题、提示词和模型会展示在灵感墙中。',
  consent: '我确认公开此作品及其提示词。',
  withdraw: '撤回发布', withdrawNotice: '从灵感墙移除此作品并停用公开媒体链接？本地副本仍会保留，其他人已下载的副本无法收回。',
  withdrawFailed: '撤回发布失败', withdrawing: '撤回中...',
  fileInvalid: '请选择不超过 64 MiB 的 PNG、JPEG、GIF、WebP、MP4 或 WebM 文件。',
  required: '请输入标题并确认公开可见。', publicLink: '打开公开媒体',
  checking: '正在查询发布状态...', unknown: '暂时无法确认发布状态。重试会查询或完成同一次发布。',
  mine: '本人发布',
  withdrawn: '此作品已撤回发布。',
  pending: '发布尚未完成，当前并未公开。',
  resume: '继续公开发布',
  cleanup: '取消待完成发布',
  cleanupNotice: '取消这次待完成发布并清理已上传的服务器文件？本地作品仍会保留。',
  cleanupFailed: '服务器文件清理失败，可重试取消发布。',
  pendingWithdrawn: '已取消公开发布，可重试清理服务器文件。',
}

export function usePublicationText() {
  const { locale } = useI18n()
  return (key: keyof typeof en) => (locale.value.startsWith('zh') ? zh : en)[key]
}
