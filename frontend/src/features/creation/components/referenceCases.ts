// Complete explicitly CC-BY-4.0 cases/ collection at revision 3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596.
// Images are resized JPEG previews. Original attribution records and LICENSE ship alongside them.
export type ReferenceCategory = 'product' | 'poster' | 'illustration' | 'architecture' | 'photography'

export interface ReferenceCase {
  id: string
  category: ReferenceCategory
  title: { zh: string; en: string }
  prompt: { zh: string; en: string }
  promptAuthor: string
  promptAuthorUrl: string
  imageAuthor: string
  imageAuthorUrl: string
  sourceUrl: string
  repositoryUrl: string
  attributionUrl: string
  license: 'CC-BY-4.0'
  creationTool: string
  image: string
  width: number
  height: number
}

export const referenceCases: ReferenceCase[] = [
  {
    "id": "gpt4o-90",
    "category": "product",
    "title": {
      "zh": "品牌化键盘键帽",
      "en": "Branded Mechanical Keycaps"
    },
    "prompt": {
      "zh": "一个超逼真的3D渲染图，展示了四个机械键盘键帽，排列成紧密的2x2网格，所有键帽相互接触。从等轴测角度观察。一个键帽是透明的，上面用红色印刷着“{just}”字样。另外三个键帽采用颜色：{黑色、紫色和白色}。一个键帽上带有Github的Logo。另外两个键帽上分别写着“{fork}”和“{it}”。逼真的塑料纹理，圆润的雕刻键帽，柔和的阴影，干净的浅灰色背景。",
      "en": "ultra-realistic 3D render of four mechanical keyboard keycaps in a tight 2x2 grid, all keys touching. View from an isometric angle. One key is transparent with the word “{just}” printed in {white}. The other three colors are: {black, purple, and white}. One key features the {Github} logo. The other two say \"{fork}\" and \"{it}\". Realistic plastic texture, rounded sculpted keycaps, soft shadows, clean light-gray background."
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1918291652210311278",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/90",
    "attributionUrl": "/creation/inspiration/attribution/case-90.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-90.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-59",
    "category": "poster",
    "title": {
      "zh": "彩色矢量艺术海报",
      "en": "Colorful Vector Art Poster"
    },
    "prompt": {
      "zh": "地点是\"英国伦敦\"，生成一张夏季的彩色矢量艺术海报，顶部有大的\"LONDON\"标题，下方有较小的\"UNITED KINGDOM\"标题",
      "en": "Barcelona Spain colourful summer vector art poster with big \"BARCELONA\" title at the top and smaller \"SPAIN\" title under"
    },
    "promptAuthor": "@michaelrabone",
    "promptAuthorUrl": "https://x.com/michaelrabone",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/michaelrabone/status/1913865394139316291",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/59",
    "attributionUrl": "/creation/inspiration/attribution/case-59.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-59.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-79",
    "category": "architecture",
    "title": {
      "zh": "乐高城市景观",
      "en": "Lego Cityscape (Shanghai Bund)"
    },
    "prompt": {
      "zh": "创建一幅高度精细且色彩鲜艳的乐高版上海外滩景象。前景呈现经典的外滩历史建筑群，用乐高砖块精致还原西式与新古典主义风格的建筑立面，包括钟楼、穹顶、柱廊等细节。乐高小人们正在沿江漫步、拍照、观光，街道两旁停靠着经典样式的乐高汽车。背景是壮观的黄浦江，以蓝色半透明乐高砖拼接，江面上有乐高渡轮和游览船。对岸的浦东陆家嘴高楼林立，包括东方明珠塔、上海中心、金茂大厦和环球金融中心，这些超现代乐高摩天大楼色彩丰富、造型逼真。天空为乐高明亮蓝色，点缀少量白色乐高积木云朵，整体呈现充满活力与现代感的视觉效果。",
      "en": "Create a highly detailed and vividly colored LEGO-style scene of the Shanghai Bund. The foreground features the iconic historical buildings of the Bund, meticulously recreated with LEGO bricks in Western and neoclassical architectural styles — including clock towers, domes, and colonnades. LEGO minifigures are seen strolling along the riverfront, taking photos, and sightseeing, with classic LEGO-style cars parked along the street.\nIn the background lies the spectacular Huangpu River, assembled with translucent blue LEGO bricks. On the water, LEGO ferries and tour boats sail along. Across the river stands the skyline of Lujiazui in Pudong, including the Oriental Pearl Tower, Shanghai Tower, Jin Mao Tower, and Shanghai World Financial Center — all rendered as vibrant, lifelike LEGO skyscrapers.\nThe sky is LEGO’s signature bright blue, adorned with a few white LEGO brick clouds, creating a visual full of energy and modernity."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1917713810346872902",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/79",
    "attributionUrl": "/creation/inspiration/attribution/case-79.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-79.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-39",
    "category": "illustration",
    "title": {
      "zh": "奇幻卡通插画",
      "en": "Fantasy Cartoon Illustration"
    },
    "prompt": {
      "zh": "一个卡通风格的角色，头部是一个带笑脸的电脑显示器，穿着手套和靴子，正开心地跳跃穿过一个发光的蓝色圆形传送门，背景是一片郁郁葱葱的奇幻森林景观。森林中细节丰富，有高大的树木、蘑菇、鲜花、宁静的河流、漂浮的岛屿，以及一个充满氛围的星夜天空，天空中有多个月亮。整体采用明亮鲜艳的色彩搭配柔和光效，风格为奇幻插画风。",
      "en": "A cartoon-style character with a smiling computer monitor as its head, wearing gloves and boots, happily jumping through a glowing, blue, circular portal in a lush, fantasy forest landscape. The forest is detailed with large trees, mushrooms, flowers, a serene river, floating islands, and an atmospheric starry night sky with multiple moons. Bright, vibrant colors with soft lighting, fantasy illustration style."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1905103477879267823",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/39",
    "attributionUrl": "/creation/inspiration/attribution/case-39.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-39.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-72",
    "category": "photography",
    "title": {
      "zh": "金色吊坠项链",
      "en": "Gold Pendant Necklace"
    },
    "prompt": {
      "zh": "一张照片级写实的特写图像，展示一条由女性手握持的金质吊坠项链。吊坠上刻有 [图像 / 表情符号] 的浮雕图案，悬挂在一条抛光金链上。背景为柔和虚化的中性米色调，采用自然光照，肤色真实，风格为产品摄影，画面比例为 16:9。",
      "en": "A photorealistic close-up of a gold pendant necklace held by female hand. The pendant features a bas-relief engraving of [image /emoji]. The pendant hangs from a polished gold chain. The background is softly blurred with neutral beige tones, and natural lighting, realistic skin tones, Product photography, 16:9 aspect ratio."
    },
    "promptAuthor": "@azed_ai",
    "promptAuthorUrl": "https://x.com/azed_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/azed_ai/status/1915770501705925106",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/72",
    "attributionUrl": "/creation/inspiration/attribution/case-72.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-72.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-100",
    "category": "product",
    "title": {
      "zh": "实物与手绘涂鸦创意广告",
      "en": "Creative Ad with Real Object and Hand-Drawn Doodle"
    },
    "prompt": {
      "zh": "一则简约且富有创意的广告，设置在纯白背景上。\n一个真实的 [真实物体] 与手绘黑色墨水涂鸦相结合，线条松散而俏皮。涂鸦描绘了：[涂鸦概念及交互：以巧妙、富有想象力的方式与物体互动]。在顶部或中部加入粗体黑色 [广告文案] 文字。在底部清晰放置 [品牌标志]。视觉效果应简洁、有趣、高对比度且构思巧妙。",
      "en": "A minimalist and creative advertisement set on a clean white background.\nA real [Real Object] is integrated into a hand-drawn black ink doodle, using loose, playful lines. The [Doodle Concept] interacts with the object in a clever, imaginative way. Include bold black [Ad Copy] text at the top or center. Place the [Brand Logo] clearly at the bottom. The visual should be clean, fun, high-contrast, and conceptually smart."
    },
    "promptAuthor": "@azed_ai",
    "promptAuthorUrl": "https://x.com/azed_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/azed_ai/status/1923016036120658122",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/100",
    "attributionUrl": "/creation/inspiration/attribution/case-100.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-100.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-7",
    "category": "architecture",
    "title": {
      "zh": "个性化房间设计",
      "en": "Personalized Room Design"
    },
    "prompt": {
      "zh": "为我生成我的房间设计（床、书架、沙发、绿植、电脑桌和电脑），墙上挂着绘画，窗外是城市夜景。可爱 3d 风格，c4d 渲染，轴测图。",
      "en": "Design a cozy bedroom in a cute 3D style with C4D-quality rendering, presented in an isometric view. The room includes a bed, bookshelf, sofa, green plants, a computer desk, and a computer setup. A framed painting hangs on the wall. Outside the window, a nighttime cityscape is visible with glowing buildings and a dark sky. All furniture and objects should have a soft, rounded, stylized design to match the cute 3D aesthetic. Lighting should be warm and inviting, creating a comfortable nighttime indoor atmosphere."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1910698005193515370",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/7",
    "attributionUrl": "/creation/inspiration/attribution/case-7.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-7.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-32",
    "category": "illustration",
    "title": {
      "zh": "折叠式纸雕立体绘本",
      "en": "3D Papercraft Pop-up Book"
    },
    "prompt": {
      "zh": "多层折叠式纸雕立体绘本，放在一张书桌上，背景纯净突出主题，绘本呈现出立体翻页书般的风格，比例为3:2横版。翻开的书页呈现【魔童版哪吒大战敖丙】的场景，所有元素皆可精细折叠组合，呈现出逼真细腻的纸张折叠质感；构图统一采用正面视角，整体视觉风格梦幻唯美，色彩缤纷绚丽，充满奇幻而生动的故事氛围。",
      "en": "Multi-layered foldable paper sculpture pop-up book, placed on a desk, with a clean background highlighting the main subject. The book presents a 3D flip-book style, with a 2:3 vertical aspect ratio. The open pages display the scene of [Nezha Demon Child version battling Ao Bing]. All elements are finely foldable and assembled, showcasing a realistic and delicate texture of folded paper. The composition uniformly adopts a frontal perspective, with an overall dreamy and beautiful visual style, vibrant and gorgeous colors, full of a fantastical and lively story atmosphere."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1923264349050675329",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/32",
    "attributionUrl": "/creation/inspiration/attribution/case-32.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-32.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-9",
    "category": "poster",
    "title": {
      "zh": "极简未来主义海报",
      "en": "Minimalist Futurist Poster"
    },
    "prompt": {
      "zh": "一张纵向（3∶4）4K 分辨率的极简未来主义展览海报，背景为超浅冷灰 #f4f4f4。\n\n海报中心有一枚流体 3D metaball，形态为【立体可口可乐经典汽水瓶】，材质磨砂玻璃并带细腻颗粒噪点。 流体渐变：Coca-Cola 红 #E41C23 → 珍珠白 #FFFFFF，呈现丝滑玻璃质感。\n\n高位 softbox 柔光照明，投射长而柔的彩色阴影与淡淡光晕。\n\n流体叠在文字之上，被遮挡的字母透过磨砂玻璃呈轻微高斯模糊。\n\n· 主标题 “Coca-Cola” 经典红色 logo 位于中部，被唯一的流体部分遮挡；被遮挡的字母透过磨砂玻璃呈轻微高斯模糊。\n\n· 副标题，Modern sans-serif 粗体全大写纯黑字体： “TASTE THE FEELING” 位于主标题下方，同样被流体局部覆盖并产生模糊，其余部分锐利。\n\n整体留白干净、构图平衡、焦点锐利、HDR 高动态范围。",
      "en": "A vertical (3:4) 4K-resolution minimalist futurist exhibition poster with an ultra-light cool gray background (#f4f4f4).\n\nAt the center of the poster is a fluid 3D metaball shaped like a classic Coca-Cola bottle in full form, rendered in frosted glass with delicate grainy noise.\nThe fluid gradient transitions from Coca-Cola Red (#E41C23) to Pearl White (#FFFFFF), giving it a silky glass-like appearance.\n\nHigh-position softbox lighting casts long, soft colored shadows and a subtle halo.\n\nThe fluid overlaps with the text: letters obscured by the frosted glass appear with a gentle Gaussian blur.\n•The main title, the classic red “Coca-Cola” logo, is centered and partially obscured by the fluid. The covered letters are slightly blurred through the frosted glass.\n•The subtitle, in bold all-caps modern sans-serif pure black font, reads: “TASTE THE FEELING”, placed below the main title. It is also partially overlapped by the fluid and blurred in those areas, while the rest remains sharp.\n\nThe overall layout is clean with generous whitespace, balanced composition, sharp focus, and HDR high dynamic range."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1921906728763105394",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/9",
    "attributionUrl": "/creation/inspiration/attribution/case-9.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-9.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-86",
    "category": "photography",
    "title": {
      "zh": "双重曝光",
      "en": "Double Exposure"
    },
    "prompt": {
      "zh": "双重曝光，Midjourney 风格，融合、混合、叠加的双重曝光图像，双重曝光风格。一幅由 Yukisakura 创作的杰出杰作，展现了一个奇妙的双重曝光构图，将阿拉贡·阿拉松之子的剪影与生机勃勃春季里中土世界视觉上引人注目、崎岖的地貌和谐地交织在一起。沐浴阳光的松树林、山峰和一匹孤独的马穿过小径的景象从他身形的纹理中向外回响，增添了叙事和孤独的层次感。当简洁分明的单色背景保持着锐利的对比度时，美妙的张力逐渐形成，将所有焦点吸引到层次丰富的双重曝光上。其特点是阿拉贡剪影内部充满活力的全彩色方案，以及用情感的精确性描摹每个轮廓的清晰、刻意的线条。(Detailed:1.45). (Detailed background:1.4).",
      "en": "Double exposure, Midjourney style, merging, blending, overlay double exposure image, Double Exposure style, An exceptional masterpiece by Yukisakura revealing a fantastic double exposure composition of Aragorn son of Arathorn's silhouette harmoniously intertwined with the visually striking, rugged landscapes of Middle Earth during a lively spring season. Sun-bathed pine forests, mountain peaks, and a lone horse cutting through the trail echo outward through the fabric of his figure, adding layers of narrative and solitude. Beautiful tension builds as the stark monochrome background maintains razor-sharp contrast, drawing all focus to the richly layered double exposure. Characterized by its vibrant full-color scheme within Aragorn's silhouette and crisp, deliberate lines that trace every contour with emotional precision. (Detailed:1.45). (Detailed background:1.4)."
    },
    "promptAuthor": "rezzycheck (Sora)",
    "promptAuthorUrl": "https://sora.com/explore?user=rezzycheck",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://sora.com/g/gen_01jtc9btfzef080z31v8w9rtbw",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/86",
    "attributionUrl": "/creation/inspiration/attribution/case-86.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-86.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-63",
    "category": "product",
    "title": {
      "zh": "Emoji 奶油雪糕",
      "en": "Emoji Cream Popsicle"
    },
    "prompt": {
      "zh": "生成图片：将【🍓】变成变成一根奶油雪糕，奶油在雪糕顶上呈曲线流动状看起来美味可口，45度悬浮在空中，q版 3d 可爱风格，一致色系的纯色背景",
      "en": "Generate an image: Transform the [🍓] into a creamy ice cream bar, with cream flowing in curved swirls on top, making it look delicious and tempting. The ice cream is floating at a 45-degree angle in mid-air, rendered in a cute chibi-style 3D aesthetic, set against a solid color background with a unified color palette."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1914574278911000967",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/63",
    "attributionUrl": "/creation/inspiration/attribution/case-63.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-63.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-71",
    "category": "architecture",
    "title": {
      "zh": "迷你 Cyberpunk 傾斜移軸景觀",
      "en": "Miniature Cyberpunk Tilt-Shift Landscape"
    },
    "prompt": {
      "zh": "從上方俯瞰的超高細節迷你【Cyberpunk】景觀，採用傾斜移軸鏡頭效果。場景中充滿如玩具般的元素，全部以高解析度 CG 呈現。光線戲劇化，營造出大片的氛圍，色彩鮮明，對比強烈，強調景深效果與擬真微觀視角，使觀者仿佛俯瞰一個玩具世界般的迷你現實，畫面中包含大量視覺笑點與極具重複觀看價值的細節設計",
      "en": "A highly detailed miniature [Cyberpunk] landscape viewed from above, using a tilt-shift lens effect. The scene is filled with toy-like elements, all rendered in high-resolution CG. Dramatic lighting creates a cinematic atmosphere, with vivid colors and strong contrast, emphasizing depth of field and a realistic micro-perspective, making the viewer feel as if overlooking a toy world. The image contains many visual jokes and details worth repeated viewing."
    },
    "promptAuthor": "terry623",
    "promptAuthorUrl": "https://github.com/terry623",
    "imageAuthor": "terry623",
    "imageAuthorUrl": "https://github.com/terry623",
    "sourceUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/71",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/71",
    "attributionUrl": "/creation/inspiration/attribution/case-71.yml",
    "license": "CC-BY-4.0",
    "creationTool": "GPT-4o",
    "image": "/creation/inspiration/case-71.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-3",
    "category": "poster",
    "title": {
      "zh": "复古宣传海报",
      "en": "Retro Style Promotional Poster"
    },
    "prompt": {
      "zh": "复古宣传海报风格，突出中文文字，背景为红黄放射状图案。画面中心位置有一位美丽的年轻女性，以精致复古风格绘制，面带微笑，气质优雅，具有亲和力。主题是GPT最新AI绘画服务的广告促销，强调‘惊爆价9.9/张’、‘适用各种场景、图像融合、局部重绘’、‘每张提交3次修改’、‘AI直出效果，无需修改’，底部醒目标注‘有意向点右下“我想要”’，右下角绘制一个手指点击按钮动作，左下角展示OpenAI标志。",
      "en": "A retro-style promotional poster emphasizing bold Chinese text. The background features a red-and-yellow radial burst pattern. In the center of the composition is a beautiful young woman illustrated in a refined vintage art style—she smiles warmly with a graceful, approachable presence. The poster advertises GPT’s latest AI image generation service with key slogans in Chinese, such as: “Shocking price: 9.9 per image”, “Supports all scenes, image blending, partial redrawing”, “3 revisions per image”, and “Direct AI output with no need for manual edits”. At the bottom, prominently display the call-to-action: “If you’re interested, click ‘I want this’ in the bottom-right corner”. Illustrate a hand pressing a button in the bottom-right, and place the OpenAI logo in the bottom-left."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1905251524248248650",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/3",
    "attributionUrl": "/creation/inspiration/attribution/case-3.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-3.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-30",
    "category": "photography",
    "title": {
      "zh": "35mm 胶片风格飞岛",
      "en": "35mm Film Style Flying Island"
    },
    "prompt": {
      "zh": "35 毫米胶片风格的照片：莫斯科漂浮在天空中的飞行岛屿上。",
      "en": "35 mm photo of Moscow floating in the sky on a flying islands."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1905020833451348283",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/30",
    "attributionUrl": "/creation/inspiration/attribution/case-30.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-30.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-99",
    "category": "photography",
    "title": {
      "zh": "黑白肖像艺术",
      "en": "Black and White Portrait Art"
    },
    "prompt": {
      "zh": "高分辨率的黑白肖像艺术作品，采用编辑类和艺术摄影风格。背景呈现柔和渐变效果，从中灰过渡到近乎纯白，营造出层次感与寂静氛围。细腻的胶片颗粒质感为画面增添了一种可触摸的、模拟摄影般的柔和质地，让人联想到经典的黑白摄影。\n\n画面右侧，一个模糊却惊艳的哈利波特面容从阴影中隐约浮现，并非传统的摆拍，而像是被捕捉于思索或呼吸之间的瞬间。他的脸部只露出一部分：也许是一个眼睛、一块颧骨，还有唇角的轮廓，唤起神秘、亲密与优雅之感。他的五官精致而深刻，散发出忧郁与诗意之美，却不显矫饰。\n\n一束温柔的定向光，柔和地漫射开来，轻抚他的面颊曲线，或在眼中闪现光点——这是画面的情感核心。其余部分以大量负空间占据，刻意保持简洁，使画面自由呼吸。画面中没有文字、没有标志——只有光影与情绪交织。\n\n整体氛围抽象却深具人性，仿佛一瞥即逝的目光，或半梦半醒间的记忆：亲密、永恒、令人怅然的美。",
      "en": "A high-resolution black and white portrait artwork, in an editorial and fine art photography style. The background features a soft gradient, transitioning from mid-gray to almost pure white, creating a sense of depth and tranquility. Fine film grain adds a tactile, analog-like softness to the image, reminiscent of classic black and white photography.\n\nOn the right side of the frame, a blurred yet striking face of Harry Potter subtly emerges from the shadows, not in a traditional pose, but as if caught in a moment of thought or breath. Only a part of his face is visible: perhaps an eye, a cheekbone, the contour of his lips, evoking a sense of mystery, intimacy, and elegance. His features are delicate yet profound, exuding a melancholic and poetic beauty without being overly dramatic.\n\nA gentle, directional light, softly diffused, caresses the curve of his cheek or glints in his eye—this is the emotional core of the image. The rest of the composition is dominated by ample negative space, intentionally kept simple, allowing the image to breathe. There are no texts, no logos in the image—only an interplay of light, shadow, and emotion.\n\nThe overall atmosphere is abstract yet deeply human, like a fleeting glance or a half-remembered dream: intimate, timeless, and poignantly beautiful."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1922150692145283299",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/99",
    "attributionUrl": "/creation/inspiration/attribution/case-99.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-99.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-98",
    "category": "photography",
    "title": {
      "zh": "磨砂玻璃后的虚实对比剪影",
      "en": "Blurred Silhouette Behind Frosted Glass"
    },
    "prompt": {
      "zh": "一张黑白照片，展示了一个[主体]在磨砂或半透明表面后的模糊剪影。其[部分]轮廓清晰，紧贴表面，与其余朦胧、模糊的身影形成鲜明对比。背景是柔和的灰色渐变色调，增强了神秘和艺术的氛围。",
      "en": "A black and white photograph shows the blurred silhouette of a [SUBJECT] behind a frosted or translucent surface. The [PART] is sharply defined and pressed against the surface, creating a stark contrast with the rest of the hazy, indistinct figure. The background is a soft gradient of gray tones, enhancing the mysterious and artistic atmosphere."
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1921487841634156999",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/98",
    "attributionUrl": "/creation/inspiration/attribution/case-98.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-98.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-97",
    "category": "product",
    "title": {
      "zh": "可爱温馨针织玩偶",
      "en": "Cute and Cozy Knitted Doll"
    },
    "prompt": {
      "zh": "一张特写、构图专业的照片，展示一个手工钩织的毛线玩偶被双手轻柔地托着。玩偶造型圆润，【上传图片】人物得可爱Q版形象，色彩对比鲜明，细节丰富。持玩偶的双手自然、温柔，手指姿态清晰可见，皮肤质感与光影过渡自然，展现出温暖且真实的触感。背景轻微虚化，表现为室内环境，有温暖的木质桌面和从窗户洒入的自然光，营造出舒适、亲密的氛围。整体画面传达出精湛的工艺感与被珍视的温馨情绪。",
      "en": "A close-up, professionally composed photograph showcasing a hand-crocheted yarn doll gently cradled by two hands. The doll has a rounded shape, featuring the cute chibi image of the [upload image] character, with vivid contrasting colors and rich details. The hands holding the doll are natural and gentle, with clearly visible finger postures, and natural skin texture and light/shadow transitions, conveying a warm and realistic touch. The background is slightly blurred, depicting an indoor environment with a warm wooden tabletop and natural light streaming in from a window, creating a comfortable and intimate atmosphere. The overall image conveys a sense of exquisite craftsmanship and cherished warmth."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1921148024861938077",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/97",
    "attributionUrl": "/creation/inspiration/attribution/case-97.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-97.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-96",
    "category": "product",
    "title": {
      "zh": "定制动漫手办",
      "en": "Custom Anime Figure"
    },
    "prompt": {
      "zh": "生成一张摆放于桌面上的动漫风格手办照片，以日常随手用手机拍摄的轻松休闲视角呈现。手办模型以附件中人物照片为基础，精确还原照片中人物的全身姿势、面部表情以及服装造型，确保手办全身完整呈现。整体设计精致细腻，头发与服饰采用自然柔和的渐变色彩与细腻质感，风格偏向日系动漫风，细节丰富，质感真实，观感精美。",
      "en": "Generate an anime-style figure photo placed on a desktop, presented from a casual, everyday snapshot perspective as if taken with a mobile phone. The figure model is based on the attached character photo, accurately reproducing the full body posture, facial expression, and clothing style of the person in the photo, ensuring the entire figure is fully rendered. The overall design is exquisite and detailed, with hair and clothing featuring natural, soft gradient colors and fine textures. The style leans towards Japanese anime, rich in detail, with realistic textures and a beautiful appearance."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1920851135516082246",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/96",
    "attributionUrl": "/creation/inspiration/attribution/case-96.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-96.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-95",
    "category": "product",
    "title": {
      "zh": "自拍生成摇头娃娃",
      "en": "Bobblehead Generator from Selfie"
    },
    "prompt": {
      "zh": "将这张照片变成一个摇头娃娃：头部稍微放大，保持面部准确，身体卡通化。[把它放在书架上]。",
      "en": "Turn this photo into a bobblehead: enlarge the head slightly, keep the face accurate and cartoonify the body. [Place it on a bookshelf]."
    },
    "promptAuthor": "@thisdudelikesAI",
    "promptAuthorUrl": "https://x.com/thisdudelikesAI",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/thisdudelikesAI/status/1920433372243136730",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/95",
    "attributionUrl": "/creation/inspiration/attribution/case-95.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-95.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-94",
    "category": "photography",
    "title": {
      "zh": "三只动物与地标自拍",
      "en": "Three Animals Selfie at Landmark"
    },
    "prompt": {
      "zh": "三只[动物类型]在标志性[地标]前的特写自拍照，它们表情各异，拍摄于黄金时刻，采用电影般的灯光。动物们靠近镜头，头挨着头，模仿自拍姿势，展现出喜悦、惊讶和平静的表情。背景展示了[地标]完整的建筑细节，光线柔和，氛围温暖。采用摄影感、写实卡通风格拍摄，高细节，1:1 宽高比。",
      "en": "A close-up selfie of three [animal type] with different expressions in front of the iconic [landmark], taken at golden hour with cinematic lighting. The animals are positioned close to the camera with their heads touching, mimicking a selfie pose, showing joyful, surprised, and calm expressions. The background features the full architectural detail of [landmark], softly illuminated, with a warm ambient atmosphere. Shot in a photographic, realistic cartoon style, high detail, 1:1 aspect ratio."
    },
    "promptAuthor": "@berryxia_ai",
    "promptAuthorUrl": "https://x.com/berryxia_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/berryxia_ai/status/1920795648946782583",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/94",
    "attributionUrl": "/creation/inspiration/attribution/case-94.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-94.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-93",
    "category": "product",
    "title": {
      "zh": "玻璃质感重塑",
      "en": "Glass Retexturing"
    },
    "prompt": {
      "zh": "对参考图片进行重新纹理化，基于下方的 JSON 美学定义\n{\n  \"style\": \"photorealistic 3D render\",\n  \"material\": \"glass with transparent and iridescent effects\",\n  \"surface_texture\": \"smooth, polished with subtle reflections and refractive effects\",\n  \"lighting\": {\n    \"type\": \"studio HDRI\",\n    \"intensity\": \"high\",\n    \"direction\": \"angled top-left key light and ambient fill\",\n    \"accent_colors\": [\"blue\", \"green\", \"purple\"],\n    \"reflections\": true,\n    \"refractions\": true,\n    \"dispersion_effects\": true,\n    \"bloom\": true\n  },\n  \"color_scheme\": {\n    \"primary\": \"transparent with iridescent blue, green, and purple hues\",\n    \"secondary\": \"crystal-clear with subtle chromatic shifts\",\n    \"highlights\": \"soft, glowing accents reflecting rainbow-like effects\",\n    \"rim_light\": \"soft reflective light around edges\"\n  },\n  \"background\": {\n    \"color\": \"black\",\n    \"vignette\": true,\n    \"texture\": \"none\"\n  },\n  \"post_processing\": {\n    \"chromatic_aberration\": true,\n    \"glow\": true,\n    \"high_contrast\": true,\n    \"sharp_details\": true\n  }\n}",
      "en": "retexture the image attached based on the JSON aesthetic below\n{\n  \"style\": \"photorealistic 3D render\",\n  \"material\": \"glass with transparent and iridescent effects\",\n  \"surface_texture\": \"smooth, polished with subtle reflections and refractive effects\",\n  \"lighting\": {\n    \"type\": \"studio HDRI\",\n    \"intensity\": \"high\",\n    \"direction\": \"angled top-left key light and ambient fill\",\n    \"accent_colors\": [\"blue\", \"green\", \"purple\"],\n    \"reflections\": true,\n    \"refractions\": true,\n    \"dispersion_effects\": true,\n    \"bloom\": true\n  },\n  \"color_scheme\": {\n    \"primary\": \"transparent with iridescent blue, green, and purple hues\",\n    \"secondary\": \"crystal-clear with subtle chromatic shifts\",\n    \"highlights\": \"soft, glowing accents reflecting rainbow-like effects\",\n    \"rim_light\": \"soft reflective light around edges\"\n  },\n  \"background\": {\n    \"color\": \"black\",\n    \"vignette\": true,\n    \"texture\": \"none\"\n  },\n  \"post_processing\": {\n    \"chromatic_aberration\": true,\n    \"glow\": true,\n    \"high_contrast\": true,\n    \"sharp_details\": true\n  }\n}"
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1920448389960909085",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/93",
    "attributionUrl": "/creation/inspiration/attribution/case-93.yml",
    "license": "CC-BY-4.0",
    "creationTool": "GPT-4o",
    "image": "/creation/inspiration/case-93.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-92",
    "category": "illustration",
    "title": {
      "zh": "透视3D出屏效果",
      "en": "Perspective 3D Pop-Out Effect"
    },
    "prompt": {
      "zh": "超写实，从上往下俯视角拍摄，一个美丽的ins模特【安妮海瑟薇 / 见参考图片】，有着精致美丽的妆容和时尚的造型，站在一部被人托起的智能手机屏幕上，画面营造出强烈的透视错觉。强调女孩从手机中站出来的三维效果。她戴着黑框眼镜，穿着高街风，俏皮地摆着可爱的pose。手机屏幕被处理成深色地板，像是一个小舞台。场景使用强烈的强制透视（forced perspective）表现手掌、手机与女孩之间的比例差异。背景为干净的灰色，使用柔和室内光，浅景深，整体风格为超现实写实合成。透视特别强",
      "en": "Hyperrealistic, top-down bird's-eye view shot, a beautiful Instagram model [Anne Hathaway / see reference image], with exquisite and beautiful makeup and fashionable styling, standing on the screen of a smartphone held up by someone. The image creates a strong perspective illusion. Emphasize the 3D effect of the girl standing out from the phone. She wears black-rimmed glasses, high-street fashion, and strikes a cute, playful pose. The phone screen is treated as a dark floor, like a small stage. The scene uses strong forced perspective to show the proportional difference between the hand, the phone, and the girl. The background is clean gray, using soft indoor light, shallow depth of field, and the overall style is surrealistic photorealistic compositing. Very strong perspective."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1920355982703509588",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/92",
    "attributionUrl": "/creation/inspiration/attribution/case-92.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-92.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-91",
    "category": "architecture",
    "title": {
      "zh": "谷歌地图变身古代藏宝图",
      "en": "Google Maps to Ancient Treasure Map"
    },
    "prompt": {
      "zh": "将图像转换为绘制在古老羊皮纸上的古代藏宝图。地图包含详细的元素，如海洋上的帆船、海岸线上的古老港口或城堡、通向标记宝藏地点的大“X”的虚线路径、山脉、棕榈树和装饰性的罗盘玫瑰。整体风格让人联想到旧时的海盗冒险电影。",
      "en": "Transform the image to an ancient treasure map drawn on aged parchment. The map includes detailed elements like sailing ships on the ocean, old ports or castles on the coastline, a dotted path leading to a large 'X' marking the treasure spot, mountains, palm trees, and a decorative compass rose. The overall style is reminiscent of old pirate adventure films."
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1919701229363466328",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/91",
    "attributionUrl": "/creation/inspiration/attribution/case-91.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-91.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-89",
    "category": "product",
    "title": {
      "zh": "镀铬emoji徽章",
      "en": "Chrome Emoji Pin"
    },
    "prompt": {
      "zh": "高精度的 3D 渲染图，按照 emoji 图标 {👍} 展示一个金属质感的徽章，固定在竖直的商品卡片上，具有超光滑的镀铬质感和圆润的 3D 图标造型，风格化的未来主义设计，带有柔和的反光与干净的阴影。纸质卡片顶部中央带有一个冲切的欧式挂孔，徽章上方是醒目的标题 “{Awesome}”，下方配有趣味标语 “{Smash that ⭐ if you like it!}”。背景为柔和的灰色，使用柔光摄影棚灯光，整体风格极简。",
      "en": "highly detailed 3D render of a single metallic {👍} emoji pin attached to a vertical product card, ultra-glossy chrome finish, smooth rounded 3D icon, stylized futuristic design, soft reflections, clean shadows, paper card has a die-cut euro hole at the top center, bold title “{Awesome}” above the pin, fun tagline “{Smash that ⭐ if you like it!}” below, soft gray background, soft studio lighting, minimal aesthetic"
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1919398870867440124",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/89",
    "attributionUrl": "/creation/inspiration/attribution/case-89.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-89.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-88",
    "category": "illustration",
    "title": {
      "zh": "儿童涂色页插画（含彩色参考图）",
      "en": "Children's Coloring Page Illustration (with Color Reference)"
    },
    "prompt": {
      "zh": "一张黑白线描涂色插画，适合直接打印在标准尺寸（8.5x11英寸）的纸张上，无纸张边框。整体插画风格清新简洁，使用清晰流畅的黑色轮廓线条，无阴影、无灰阶、无颜色填充，背景纯白，便于涂色。\n【同时为了方便不会涂色的用户，请在右下角用小图生成一个完整的彩色版本供参考】\n适合人群：【6-9岁小朋友】\n画面描述：\n【一只独角兽在森林的草地上漫步，阳光明媚，蓝天白云】",
      "en": "A black and white line drawing coloring illustration, suitable for direct printing on standard size (8.5x11 inch) paper, without paper borders. The overall illustration style is fresh and simple, using clear and smooth black outline lines, without shadows, grayscale, or color filling, with a pure white background for easy coloring.\n[At the same time, for the convenience of users who are not good at coloring, please generate a complete colored version in the lower right corner as a small image for reference]\nSuitable for: [6-9 year old children]\nScene description:\n[A unicorn is walking on the grass in the forest, with bright sunshine, blue sky and white clouds]"
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1919522110395080838",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/88",
    "attributionUrl": "/creation/inspiration/attribution/case-88.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-88.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-87",
    "category": "poster",
    "title": {
      "zh": "字母与单词含义融合",
      "en": "Integrating Word Meaning into Letters"
    },
    "prompt": {
      "zh": "在字母中融入单词的含义，将图形和字母巧妙融合在一起。\n单词：{ beautify }\n下面加上单词的简要说明",
      "en": "Integrate the meaning of the word into the letters, cleverly blending graphics and letters.\nWord: {beautify}\nAdd a brief explanation of the word below."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1918529055340576812",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/87",
    "attributionUrl": "/creation/inspiration/attribution/case-87.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-87.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-85",
    "category": "photography",
    "title": {
      "zh": "超现实交互场景",
      "en": "Surreal Interaction Scene"
    },
    "prompt": {
      "zh": "一幅铅笔素描画，描绘了 [Subject 1] 与 [Subject 2] 互动的场景，其中 [Subject 2] 以逼真的全彩风格呈现，与 [Subject 1] 及背景的手绘素描风格形成超现实的对比。",
      "en": "A pencil sketch of [Subject 1] interacting with [Subject 2], where [Subject 2] is rendered as a realistic, full-color object, creating a surreal contrast against the hand-drawn style of [Subject 1] and the background"
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1917444534239191544",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/85",
    "attributionUrl": "/creation/inspiration/attribution/case-85.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-85.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-84",
    "category": "product",
    "title": {
      "zh": "动物硅胶腕托",
      "en": "Animal Silicone Wrist Rest"
    },
    "prompt": {
      "zh": "创建图片 一个可爱Q版的硅胶护腕托，外形基于【🐼】表情，采用柔软的食品级硅胶材质，表面为亲肤哑光质感，内部填充慢回弹棉，拟人化卡通风格，表情生动，双手张开趴在桌面上，呈现出拥抱手腕的姿势，整体造型圆润软萌，颜色为【🐼】配色，风格治愈可爱，适合办公使用，背景为白色纯色，柔和布光，产品摄影风格，前视角或45度俯视，高清细节，突出硅胶质感与舒适功能",
      "en": "Create an image of a cute chibi-style silicone wrist rest based on the {🐼} emoji. The wrist rest is made of soft, food-grade silicone with a skin-friendly matte surface. The interior is filled with slow-rebound foam. Designed in a personified cartoon style, the expression is lively, with both arms stretched out as if hugging the user’s wrist while lying on a desk. The overall shape is round, soft, and adorable, featuring the classic {🐼} color scheme. The design is comforting and cute, suitable for office use. The background is a solid white color with soft lighting. Rendered in a product photography style, the angle is either front-facing or at a 45-degree top-down view, showcasing high-definition details and emphasizing the silicone texture and comfort functionality."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1918525296577327574",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/84",
    "attributionUrl": "/creation/inspiration/attribution/case-84.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-84.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-83",
    "category": "illustration",
    "title": {
      "zh": "发光线条解剖图",
      "en": "Glowing Lines Anatomy Diagram"
    },
    "prompt": {
      "zh": "一幅数字插画，描绘了一个 [SUBJECT]，其结构由一组发光、干净且纯净的蓝色线条勾勒而成。画面设定在深色背景之上，以突出 [SUBJECT] 的形态与特征。某个特定部位，如 [PART]，通过红色光晕加以强调，以表示该区域的重要性或特殊意义。整体风格兼具教育性与视觉吸引力，设计上仿佛是一种先进的成像技术。",
      "en": "A digital illustration of a [SUBJECT], portrayed with a network of glowing clean pristine blue lines outlining its anatomy. The image is set against a dark background, highlighting the [SUBJECT] form and features. A specific area such as [PART] is emphasized with a red glow to indicate a point of interest or significance. The style is both educational and visually captivating, designed to resemble an advanced imaging technique"
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1914644426334314545",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/83",
    "attributionUrl": "/creation/inspiration/attribution/case-83.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-83.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-82",
    "category": "architecture",
    "title": {
      "zh": "特色城市天气预报",
      "en": "Signature City Weather Forecast"
    },
    "prompt": {
      "zh": "以清晰的45°俯视角度，展示一个等距微缩模型场景，内容为[上海东方明珠塔、外滩]等城市特色建筑，天气效果巧妙融入场景中，柔和的多云天气与城市轻柔互动。使用基于物理的真实渲染（PBR）和逼真的光照效果，纯色背景，清晰简洁。画面采用居中构图，凸显出三维模型精准而细腻的美感。在图片上方展示“[上海 多云 20°C]”，并附有多云天气图标。",
      "en": "Show a clear 45-degree bird’s-eye view of an isometric miniature city scene featuring Shanghai’s iconic buildings, such as the Oriental Pearl Tower and the Bund. The weather effect—cloudy—blends softly into the city, interacting gently with the architecture. Use physically based rendering (PBR) and realistic lighting. Solid color background, crisp and clean. Centered composition to highlight the precision and detail of the 3D model. Display “Shanghai Cloudy 20°C” and a cloudy weather icon at the top of the image."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1917988595228438771",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/82",
    "attributionUrl": "/creation/inspiration/attribution/case-82.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-82.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-81",
    "category": "product",
    "title": {
      "zh": "半透明玻璃质感变换",
      "en": "3D Translucent Glass Transformation"
    },
    "prompt": {
      "zh": "将附图变换为柔软的3D半透明玻璃，具有磨砂哑光效果和细致的纹理，原始色彩，以浅灰色背景为中心，在空间中轻轻漂浮，柔和的阴影，自然的光线",
      "en": "A soft, 3D translucent glass of the attached image with a frosty matte finish and detailed texture, original colors, centered on a light gray background, floats gently in space, soft shadows, natural lighting"
    },
    "promptAuthor": "@azed_ai",
    "promptAuthorUrl": "https://x.com/azed_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/azed_ai/status/1917948899098243407",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/81",
    "attributionUrl": "/creation/inspiration/attribution/case-81.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-81.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-80",
    "category": "poster",
    "title": {
      "zh": "代码风格名片",
      "en": "Code Style Business Card"
    },
    "prompt": {
      "zh": "特写镜头：一只手正拿着一张设计成 VS Code 中 JSON 文件外观的名片。名片上的代码以真实的 JSON 语法高亮格式呈现。窗口界面包含典型的工具栏图标和标题栏，标题显示为 Business Card.json，整体风格与 VS Code 界面完全一致。背景略微虚化，突出展示名片内容。\n名片上的 JSON 代码如下所示：\n{\n  \"name\": \"Jamez Bondos\",\n  \"title\": \"Your Title\",\n  \"email\": \"your@email.com\",\n  \"link\": \"yourwebsite\"\n}",
      "en": "A close-up shot of a hand holding a business card designed to look like a JSON file opened in VS Code. The card shows code formatted in realistic syntax-highlighted JSON code. The window includes typical toolbar icons and a title bar labeled Business Card.json, styled exactly like the interface of VS Code. Background is slightly blurred, keeping the focus on the card.\nThe card displays the following code formatted in JSON:\n{\n  \"name\": \"Jamez Bondos\",\n  \"title\": \"Your Title\",\n  \"email\": \"your@email.com\",\n  \"link\": \"yourwebsite\"\n}"
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1915696926596415492",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/80",
    "attributionUrl": "/creation/inspiration/attribution/case-80.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-80.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-78",
    "category": "product",
    "title": {
      "zh": "玻璃材质重塑",
      "en": "Glass Retexturing"
    },
    "prompt": {
      "zh": "retexture the image attached based on the json below:\n\n{\n  \"style\": \"photorealistic\",\n  \"material\": \"glass\",\n  \"background\": \"plain white\",\n  \"object_position\": \"centered\",\n  \"lighting\": \"soft, diffused studio lighting\",\n  \"camera_angle\": \"eye-level, straight-on\",\n  \"resolution\": \"high\",\n  \"aspect_ratio\": \"2:3\",\n  \"details\": {\n    \"reflections\": true,\n    \"shadows\": false,\n    \"transparency\": true\n  }\n}",
      "en": "retexture the image attached based on the json below:\n\n{\n  \"style\": \"photorealistic\",\n  \"material\": \"glass\",\n  \"background\": \"plain white\",\n  \"object_position\": \"centered\",\n  \"lighting\": \"soft, diffused studio lighting\",\n  \"camera_angle\": \"eye-level, straight-on\",\n  \"resolution\": \"high\",\n  \"aspect_ratio\": \"2:3\",\n  \"details\": {\n    \"reflections\": true,\n    \"shadows\": false,\n    \"transparency\": true\n  }\n}"
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1917631056980721743",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/78",
    "attributionUrl": "/creation/inspiration/attribution/case-78.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-78.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-77",
    "category": "illustration",
    "title": {
      "zh": "水晶球故事场景",
      "en": "Story Scene in Crystal Ball"
    },
    "prompt": {
      "zh": "一枚精致的水晶球静静摆放在窗户旁温暖柔和的桌面上，背景虚化而朦胧，暖色调的阳光轻柔地穿透水晶球，折射出点点金光，温暖地照亮了四周的微暗空间。水晶球内部自然地呈现出一个以 {嫦娥奔月} 为主题的迷你立体世界，细腻精美而梦幻的3D景观，人物与物体皆是可爱的Q版造型，精致而美观，彼此之间充满灵动的情感互动。整体氛围充满了东亚奇幻色彩，细节极为丰富，呈现出魔幻现实主义般的奇妙质感。整个场景如诗如梦，华美而典雅，散发着温馨柔和的光芒，仿佛在温暖的光影中被赋予了生命。",
      "en": "A delicate crystal ball rests quietly on a warm, softly lit tabletop by the window. The background is blurred and hazy, with warm-toned sunlight gently passing through the crystal ball, refracting specks of golden light that softly illuminate the dim surroundings.\nInside the crystal ball, a miniature three-dimensional world themed around {Chang’e Flying to the Moon} is naturally displayed — a finely detailed, dreamlike 3D scene. All characters and objects are rendered in adorable chibi style, exquisitely crafted and visually charming, with vivid emotional interactions between them.\nThe overall atmosphere is rich with East Asian fantasy elements, full of intricate details and a surreal magical realism texture. The entire scene feels poetic and dreamy, luxurious yet elegant, radiating a gentle, comforting glow — as if imbued with life through the warm play of light and shadow."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1916530529324699858",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/77",
    "attributionUrl": "/creation/inspiration/attribution/case-77.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-77.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-76",
    "category": "poster",
    "title": {
      "zh": "怀旧动漫风格电影海报",
      "en": "Nostalgic Anime Film Poster"
    },
    "prompt": {
      "zh": "{The Lord of the Rings} 风格的动漫电影海报，动漫画风为《恶魔高中 DXD（High School DXD）》风格。海报上可见明显的折痕痕迹，因长时间反复折叠，造成部分区域出现褶皱处的物理性损伤和擦痕，颜色也在某些地方出现了褪色。表面遍布无规律的折痕、翻折印记与划痕，这些都是在不断搬动过程中逐渐积累的微小损耗，如同熵增不可逆的过程在不断扩展。\n然而，留存在我们心中的美好记忆却始终完整无缺。当你凝视这张充满怀旧氛围的海报时，所感受到的，正是那些随时间累积、变得无比珍贵的收藏品所承载的情感本质。",
      "en": "{The Lord of the Rings} anime film poster, the anime is in the style of High School DXD. Visible even folds are seen across the poster as it’s been folded over time, and due to some creases over damaging the poster has caused some physical damage scuffing along the creases and the color has partially faded. Indiscriminate flaps and folds and scratches all around simply from moving back and forth causing subtle yet incremental damage with the ever expanding of entropy we cannot escape, but the loving memories in our hearts will forever be whole. Making the objects we collect along the way priceless is the essence you feel when looking at this nostalgic poster."
    },
    "promptAuthor": "photis (Sora)",
    "promptAuthorUrl": "https://sora.com/explore?user=user-sydD5ZkXZsDaL0BriQa010dQ",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://sora.com/g/gen_01jsfxrdpjfpebnyed8yaz42nf",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/76",
    "attributionUrl": "/creation/inspiration/attribution/case-76.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-76.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-75",
    "category": "illustration",
    "title": {
      "zh": "社交媒体相框融合",
      "en": "Social Media Frame Integration"
    },
    "prompt": {
      "zh": "根据所附照片创建一个风格化的3D Q版人物角色，准确保留人物的面部特征和服装细节。角色的左手比心（手指上方有红色爱心元素），姿势俏皮地坐在一个巨大的Instagram相框边缘，双腿悬挂在框外。相框顶部显示用户名『Beauty』，四周漂浮着社交媒体图标（点赞、评论、转发）。",
      "en": "Create a stylized 3D chibi character based on the attached photo, accurately preserving the subject’s facial features and clothing details. The character is making a finger heart with the left hand (with a red heart element above the fingers) and playfully sitting on the edge of a giant Instagram frame, with both legs hanging outside the frame. The top of the frame displays the username “Beauty,” and various social media icons (like, comment, share) float around the scene."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1917042797506662560",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/75",
    "attributionUrl": "/creation/inspiration/attribution/case-75.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-75.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-74",
    "category": "product",
    "title": {
      "zh": "Logo 形状创意书架",
      "en": "Creative Logo Shaped Bookshelf"
    },
    "prompt": {
      "zh": "拍摄一张现代书架的照片，其造型灵感来源于 [LOGO] 的形状。书架由流畅、互相连接的曲线构成，形成多个大小不一的分区。整体材质为光滑的哑光黑色金属，曲线内部设有木质层板。柔和暖色的 LED 灯带勾勒出内侧曲线轮廓。书架安装在一个中性色调的墙面上，上面摆放着色彩丰富的书籍、小型绿植和极简风格的艺术摆件。整体氛围富有创意、优雅且略带未来感。",
      "en": "Create a photograph of a modern bookshelf inspired by the shape of [LOGO]. The bookshelf features flowing, interconnected curves forming multiple sections of varying sizes. It is made of sleek matte black metal with wooden shelves inside the loops. Soft, warm LED lighting outlines the inner curves. The bookshelf is mounted on a neutral-toned wall and holds a mix of colorful books, small plants, and minimalistic art pieces. The overall vibe is creative, elegant, and slightly futuristic"
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1916517976414495161",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/74",
    "attributionUrl": "/creation/inspiration/attribution/case-74.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-74.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-73",
    "category": "product",
    "title": {
      "zh": "定制Q版钥匙串",
      "en": "Cute Chibi Keychain"
    },
    "prompt": {
      "zh": "一张特写照片，展示一个被人手握住的可爱多彩钥匙串。钥匙串的造型为 [参考图片] 的 Q 版风格。钥匙串由柔软橡胶材质制成，带有粗黑描边，连接在一个小巧的银色钥匙圈上，背景为中性色调。",
      "en": "A close-up photo of a cute, colorful keychain held by person's hand. The keychain features a chibi-style of the [attached image ]. The keychain is made of soft rubber with bold black outlines and attached to a small silver keyring, neutral background"
    },
    "promptAuthor": "@azed_ai",
    "promptAuthorUrl": "https://x.com/azed_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/azed_ai/status/1916521742052503804",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/73",
    "attributionUrl": "/creation/inspiration/attribution/case-73.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-73.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-70",
    "category": "illustration",
    "title": {
      "zh": "原创宝可梦生成",
      "en": "Original Pokemon Creation"
    },
    "prompt": {
      "zh": "根据此物体（提供的照片）创作一个原创生物。该生物应看起来像是属于一个奇幻怪物捕捉宇宙，具有受复古日式RPG怪物艺术影响的可爱或酷炫设计。图像必须包含：\n  – 生物的全身视图，灵感来自物体的形状、材料或用途。\n  – 在其脚边有一个小球体或胶囊（类似于精灵球），其设计图案和颜色与物体的外观相匹配——不是标准的精灵球，而是自定义设计。\n  – 为生物发明的名字，显示在其旁边或下方。 – 其元素类型（例如火、水、金属、自然、电……），基于物体的核心属性。插图应看起来像是来自奇幻生物百科全书，线条清晰，阴影柔和，设计富有表现力且以角色为驱动。",
      "en": "Create an original creature inspired by this object (photo provided). The creature should look like it belongs in a fantasy monster-catching universe, with a cute or cool design influenced by retro Japanese RPG monster art. The image must include:\n– A full-body view of the creature, inspired by the shape, materials or purpose of the object.\n– A small orb or capsule (similar an a pokeball) at its feet, designed with patterns and colors matching the object’s look — not a standard Pokéball, but a custom design.\n– An invented name for the creature, displayed next to or below it. – Its elemental type (e.g., Fire, Water, Metal, Nature, Electric…), based on the object’s core properties. The illustration should look like it comes from a fantasy creature encyclopedia, with clean lines, soft shadows, and an expressive, character-driven design."
    },
    "promptAuthor": "@Anima_Labs",
    "promptAuthorUrl": "https://x.com/Anima_Labs",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/Anima_Labs/status/1915044265895379166",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/70",
    "attributionUrl": "/creation/inspiration/attribution/case-70.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-70.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-69",
    "category": "illustration",
    "title": {
      "zh": "剪影艺术",
      "en": "Silhouette Art"
    },
    "prompt": {
      "zh": "一个 [PROMPT] 的基础轮廓剪影。背景为亮黄色，剪影为纯黑色实心填充。",
      "en": "The silhouette of a basic outline of a [PROMPT]. The background is bright yellow, and the silhouette is solid black."
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1915417277953962048",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/69",
    "attributionUrl": "/creation/inspiration/attribution/case-69.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-69.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-68",
    "category": "poster",
    "title": {
      "zh": "未来主义 Logo 交易卡",
      "en": "Futuristic Logo Trading Card"
    },
    "prompt": {
      "zh": "{\n    \"prompt\": \"A futuristic trading card with a dark, moody neon aesthetic and soft sci-fi lighting. The card features a semi-transparent, rounded rectangle with slightly muted glowing edges, appearing as if made of holographic glass. At the center is a large glowing logo of {{logo}}, with no additional text or label, illuminated with a smooth gradient of {{colors}}, but not overly bright. The reflections on the card surface should be subtle, with a slight glossy finish catching ambient light. The background is a dark carbon fiber texture or deep gradient with soft ambient glows bleeding into the edges. Add subtle light rays streaming down diagonally from the top, giving the scene a soft cinematic glow. Apply light motion blur to the edges and reflections to give the scene a sense of depth and energy, as if it's part of a high-end tech animation still. Below the card, include realistic floor reflections that mirror the neon edges and logo—slightly diffused for a grounded, futuristic look. Text elements are minimal and softly lit: top-left shows '{{ticker}}', top-right has a stylized signature, and the bottom displays '{{company_name}}' with a serial number '{{card_number}}', a revenue badge reading '{{revenue}}', and the year '{{year}}'. Typography should have a faint glow with slight blurring, and all elements should feel premium, elegant, and softly illuminated—like a high-end cyberpunk collectible card.\",\n    \"style\": {\n        \"lighting\": \"Neon glow, soft reflections\",\n        \"font\": \"Modern sans-serif, clean and minimal\",\n        \"layout\": \"Centered, structured like a digital collectible card\",\n        \"materials\": \"Glass, holographic plastic, glowing metal edges\"\n    },\n    \"parameters\": {\n        \"logo\": \"Tesla logo\",\n        \"ticker\": \"TSLA\",\n        \"company_name\": \"Tesla Inc.\",\n        \"card_number\": \"#0006\",\n        \"revenue\": \"$96.8B\",\n        \"year\": \"2025\",\n        \"colors\": [\n            \"red\",\n            \"white\",\n            \"dark gray\"\n        ]\n    },\n    \"medium\": \"3D render, high-resolution digital art\",\n    \"size\": \"1080px by 1080px\"\n}",
      "en": "{\n    \"prompt\": \"A futuristic trading card with a dark, moody neon aesthetic and soft sci-fi lighting. The card features a semi-transparent, rounded rectangle with slightly muted glowing edges, appearing as if made of holographic glass. At the center is a large glowing logo of {{logo}}, with no additional text or label, illuminated with a smooth gradient of {{colors}}, but not overly bright. The reflections on the card surface should be subtle, with a slight glossy finish catching ambient light. The background is a dark carbon fiber texture or deep gradient with soft ambient glows bleeding into the edges. Add subtle light rays streaming down diagonally from the top, giving the scene a soft cinematic glow. Apply light motion blur to the edges and reflections to give the scene a sense of depth and energy, as if it's part of a high-end tech animation still. Below the card, include realistic floor reflections that mirror the neon edges and logo—slightly diffused for a grounded, futuristic look. Text elements are minimal and softly lit: top-left shows '{{ticker}}', top-right has a stylized signature, and the bottom displays '{{company_name}}' with a serial number '{{card_number}}', a revenue badge reading '{{revenue}}', and the year '{{year}}'. Typography should have a faint glow with slight blurring, and all elements should feel premium, elegant, and softly illuminated—like a high-end cyberpunk collectible card.\",\n    \"style\": {\n        \"lighting\": \"Neon glow, soft reflections\",\n        \"font\": \"Modern sans-serif, clean and minimal\",\n        \"layout\": \"Centered, structured like a digital collectible card\",\n        \"materials\": \"Glass, holographic plastic, glowing metal edges\"\n    },\n    \"parameters\": {\n        \"logo\": \"Tesla logo\",\n        \"ticker\": \"TSLA\",\n        \"company_name\": \"Tesla Inc.\",\n        \"card_number\": \"#0006\",\n        \"revenue\": \"$96.8B\",\n        \"year\": \"2025\",\n        \"colors\": [\n            \"red\",\n            \"white\",\n            \"dark gray\"\n        ]\n    },\n    \"medium\": \"3D render, high-resolution digital art\",\n    \"size\": \"1080px by 1080px\"\n}"
    },
    "promptAuthor": "@hewarsaber",
    "promptAuthorUrl": "https://x.com/hewarsaber",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/hewarsaber/status/1912933875166171515",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/68",
    "attributionUrl": "/creation/inspiration/attribution/case-68.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-68.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-67",
    "category": "illustration",
    "title": {
      "zh": "超写实3D游戏",
      "en": "Ultra-realistic 3D Game"
    },
    "prompt": {
      "zh": "超写实的 3D 渲染画面，重现了2008年《命令与征服：红色警戒3》中娜塔莎的角色设计，完全依照原版建模。场景设定在一个昏暗杂乱的2008年代卧室里，角色正坐在地毯上，面对一台正在播放《命令与征服：红色警戒3》的老式电视和游戏机手柄。\n\n整个房间充满了2008年代的怀旧氛围：零食包装袋、汽水罐、海报以及纠缠在一起的电线。娜塔莎·沃尔科娃在画面中被抓拍到转头的一瞬，回眸看向镜头，她那标志性的空灵美丽面容上带着一抹纯真的微笑。她的上半身微微扭转，动态自然，仿佛刚刚被闪光灯惊到而做出的反应。\n\n闪光灯轻微地过曝了她的脸和衣服，使她的轮廓在昏暗的房间中更加突出。整张照片显得原始而自然，强烈的明暗对比在她身后投下深邃的阴影，画面充满触感，带有一种真实的2008年胶片快照的模拟质感。",
      "en": "Ultra-realistic 3D rendered image that replicates the character design of Natasha from Command & Conquer: Red Alert 3 in 2008, following the original model exactly. The scene is set in a dim and cluttered bedroom from the year 2008. The character is sitting on the carpet, facing an old-fashioned television that is playing Command & Conquer: Red Alert 3 and a game console controller.\nThe entire room is filled with a nostalgic atmosphere of the year 2008: snack packaging bags, soda cans, posters, and tangled wires are everywhere. Natasha Volkova is captured in the moment of turning her head, looking back at the camera over her shoulder. There is an innocent smile on her iconic ethereally beautiful face. Her upper body is slightly twisted, with a natural dynamic, as if she is reacting to being startled by the flash.\nThe flash slightly overexposes her face and clothes, making her silhouette stand out more prominently in the dimly lit room. The whole photo appears raw and natural. The strong contrast between light and dark casts deep shadows behind her. The image is full of tactile feel, with a simulated texture that resembles an authentic film snapshot from 2008."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1913648013144137840",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/67",
    "attributionUrl": "/creation/inspiration/attribution/case-67.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-67.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-66",
    "category": "photography",
    "title": {
      "zh": "创意丝绸宇宙",
      "en": "Creative Silk Universe"
    },
    "prompt": {
      "zh": "将 {❄️} 变成一个柔软的 3D 丝绸质感物体。整个物体表面包裹着顺滑流动的丝绸面料，带有超现实的褶皱细节、柔和的高光与阴影。该物体轻轻漂浮在干净的浅灰色背景中央，营造出轻盈优雅的氛围。整体风格超现实、触感十足且现代，传递出舒适与精致趣味的感觉。工作室灯光，高分辨率渲染。",
      "en": "Transform the {❄️} into a soft 3D object with a silk texture. The entire surface of the object is wrapped in smooth and flowing silk fabric, featuring surreal wrinkle details, soft highlights, and shadows. The object gently floats in the center of a clean light gray background, creating a light and elegant atmosphere. The overall style is surreal, tactile, and modern, conveying a sense of comfort and refined playfulness. Studio lighting, high-resolution rendering."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1914864217867608175",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/66",
    "attributionUrl": "/creation/inspiration/attribution/case-66.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-66.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-65",
    "category": "product",
    "title": {
      "zh": "奇幻水下场景冰棒",
      "en": "Surreal Underwater Scene Popsicle"
    },
    "prompt": {
      "zh": "倾斜的第一人称视角拍摄，一只手握着一支超现实的冰棒。冰棒有着透明的蓝色外壳，里面展现了一个水下场景：一个小潜水员、几条小鱼和漂浮的气泡，还有翻滚的海浪，一根绿色的冰棒棍贯穿中心。冰棒略微融化，底部是一根木棍，手正握着这根木棍。背景是柔焦的纽约街景，采用高端产品摄影风格。",
      "en": "Tilt POV shot of a hand holding a surreal popsicle with a transparent blue exterior, revealing an underwater scene inside: a tiny scuba diver with tiny fish floating with bubbles, ocean waves crashing, and a green popsicle stick running through the center. The popsicle is melting slightly, with a wooden stick at the bottom, hand is holding it by the wooden stick, soft focus new york street background, premium product photography"
    },
    "promptAuthor": "@madpencil_",
    "promptAuthorUrl": "https://x.com/madpencil_",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/madpencil_/status/1915099250079469907",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/65",
    "attributionUrl": "/creation/inspiration/attribution/case-65.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-65.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-64",
    "category": "illustration",
    "title": {
      "zh": "蒸汽朋克机械鱼",
      "en": "Steampunk Mechanical Fish"
    },
    "prompt": {
      "zh": "一个蒸汽朋克风格的机械鱼，身体为黄铜风格，可以清楚的看到其动作时的机械齿轮结构。\n能略微看到它的机械牙齿，整齐并且紧闭，上下牙齿都可以看到。每颗牙齿均呈三角状，材质为金刚石。\n尾鳍为金属丝编织结构，其它部分的鱼鳍是半透明的琥珀色玻璃，其中有一些不太明显的气泡。\n眼睛是多面红宝石，能清晰的看到它反射出来的光泽。\n鱼有身上能清晰的看到\"f-is-h\"字样，其中字母全部为小写，并且注意横线位置。\n图片是正方形的，整个画面中可以看到鱼的全身，在画面正中，鱼头向右，并且有一定的留白画面并不局促，画面的左右留出更多的空间。背景中有淡淡的蒸汽朋克风的齿轮纹理。\n整个鱼看起非常炫酷。这是一张高清图片，整张照片的细节非常丰富，并且有独特的质感与美感。画面不要太暗。",
      "en": "A steampunk-style mechanical fish with a brass body and clearly visible gear mechanisms when in motion.\nIts mechanical teeth can be slightly seen, neatly arranged and closed, with both upper and lower teeth visible. Each tooth is triangular in shape and made of diamond material.\nThe tail fin has a metal wire mesh structure, while other fins are made of semi-transparent amber-colored glass with some subtle bubbles inside.\nThe eyes are multi-faceted rubies, with clearly visible reflective shine.\nThe fish has \"f-is-h\" text clearly visible on its body, with all lowercase letters and careful attention to the hyphen placement.\nThe image is square, showing the entire fish in the center of the frame, with its head pointing to the right. There is adequate white space around the fish, with more space on the left and right sides. The background has subtle steampunk-style gear patterns.\nThe entire fish looks very cool. This is a high-definition image with extremely rich details and unique texture and aesthetics. The image should not be too dark."
    },
    "promptAuthor": "@f-is-h",
    "promptAuthorUrl": "https://github.com/f-is-h",
    "imageAuthor": "@f-is-h",
    "imageAuthorUrl": "https://github.com/f-is-h",
    "sourceUrl": "https://github.com/f-is-h/f-is-h/blob/main/images/streampank-fish-4.png",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/64",
    "attributionUrl": "/creation/inspiration/attribution/case-64.yml",
    "license": "CC-BY-4.0",
    "creationTool": "GPT-4o",
    "image": "/creation/inspiration/case-64.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-62",
    "category": "product",
    "title": {
      "zh": "可爱珐琅别针",
      "en": "Kawaii Enamel Pin"
    },
    "prompt": {
      "zh": "将附图中的人物转换成可爱的珐琅徽章风格。使用光亮金属描边和鲜艳的珐琅填色。不添加任何额外元素。方形效果图格式，白色背景。",
      "en": "Turn the subject in the attached image into a kawaii enamel pin. Use glossy metal outlines and vibrant enamel fill. No extra added features. Square mockup format. White background."
    },
    "promptAuthor": "@gnrlyxyz",
    "promptAuthorUrl": "https://x.com/gnrlyxyz",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/gnrlyxyz/status/1914303110853583302",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/62",
    "attributionUrl": "/creation/inspiration/attribution/case-62.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-62.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-61",
    "category": "poster",
    "title": {
      "zh": "虚构推文截图 (爱因斯坦)",
      "en": "Fake Tweet Screenshot (Einstein)"
    },
    "prompt": {
      "zh": "爱因斯坦刚刚完成相对论后发布的一条超写实风格的推文。包含一张自拍照，照片中清晰可见背景中的粉笔板和潦草的公式。推文下方显示尼古拉·特斯拉点赞了该内容。",
      "en": "a hyper realistic twitter post by Albert Einstein right after finishing the theory of relativity. include a selfie where you can clearly see scribbled equations and a chalkboard in the background. have it visible that the post was liked by Nikola Tesla"
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1914299716394778713",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/61",
    "attributionUrl": "/creation/inspiration/attribution/case-61.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-61.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-60",
    "category": "product",
    "title": {
      "zh": "Emoji 簇绒地毯",
      "en": "Emoji Tufted Rug"
    },
    "prompt": {
      "zh": "创建一张图像，展示一个彩色、手工簇绒的地毯，形状为 🦖 表情符号，铺设在一个简约的地板背景上。地毯设计大胆、俏皮，具有柔软蓬松的质感和粗线条的细节。从上方俯拍，使用自然光照，整体风格略带古怪的 DIY 美感。色彩鲜艳，轮廓卡通化，材质具触感且温馨舒适——类似于手工簇绒艺术地毯。",
      "en": "Create an image of a colorful, hand-tufted rug in the shape of 🦖 emoji, placed on a simple floor background. The rug has a bold, playful design with soft, fluffy texture and thick yarn details. Shot from above, in natural daylight, with a slightly quirky, DIY aesthetic. Vibrant colors, cartoonish outlines, and tactile, cozy material—similar to handmade tufted art rugs."
    },
    "promptAuthor": "@gizakdag",
    "promptAuthorUrl": "https://x.com/gizakdag",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/gizakdag/status/1913925062568144924",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/60",
    "attributionUrl": "/creation/inspiration/attribution/case-60.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-60.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-58",
    "category": "photography",
    "title": {
      "zh": "云彩艺术",
      "en": "Cloud Art"
    },
    "prompt": {
      "zh": "生成一张照片：捕捉了白天的场景，天空中散落的云彩组成了 [主体/物体] 的形状，位于 [地点] 的上方。",
      "en": "Generate image: A photograph captures a daytime scene with a [SUBJECT/OBJECT] formed by scattered clouds in the sky, positioned above a [LOCATION]"
    },
    "promptAuthor": "@umesh_ai",
    "promptAuthorUrl": "https://x.com/umesh_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/umesh_ai/status/1913628737872027805",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/58",
    "attributionUrl": "/creation/inspiration/attribution/case-58.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-58.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-57",
    "category": "illustration",
    "title": {
      "zh": "8位像素图标",
      "en": "8-Bit Pixel Icon"
    },
    "prompt": {
      "zh": "创建一个极简主义的 8 位像素风格的 [🍔] 标志，居中放置在纯白背景上。使用有限的复古调色板，搭配像素化细节、锐利边缘和干净的块状形态。标志应简洁、具有标志性，并能在像素艺术风格中清晰识别——灵感来自经典街机游戏美学。",
      "en": "Create a minimalist 8-bit pixel logo of [🍔], centered on a pure white background. Use a limited retro color palette with pixelated detailing, sharp edges, and clean blocky forms. The logo should be simple, iconic, and clearly recognizable in pixel art style — inspired by classic arcade game aesthetics."
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1913654508330058064",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/57",
    "attributionUrl": "/creation/inspiration/attribution/case-57.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-57.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-56",
    "category": "architecture",
    "title": {
      "zh": "迷你 3D 建筑",
      "en": "Miniature 3D Building"
    },
    "prompt": {
      "zh": "3D Q版迷你风格，一个充满奇趣的迷你星巴克咖啡馆，外观就像一个巨大的外带咖啡杯，还有盖子和吸管。建筑共两层，大大的玻璃窗清晰地展示出内部温馨而精致的设计：木质的家具、温暖的灯光以及忙碌的咖啡师们。街道上有可爱的小人偶漫步或坐着，四周布置着长凳、街灯和植物盆栽，营造出迷人的城市一角。整体采用城市微缩景观风格，细节丰富、逼真，画面光线柔和、呈现出午后的惬意感受。",
      "en": "3D chibi-style miniature design of a whimsical Starbucks café, shaped like an oversized takeaway coffee cup complete with a lid and straw. The building has two floors, with large glass windows that clearly reveal a cozy and refined interior: wooden furniture, warm lighting, and busy baristas at work. On the street, cute little figurines are strolling or sitting, surrounded by benches, street lamps, and potted plants, creating a charming corner of the city. The overall aesthetic follows a detailed and realistic miniature cityscape style, with soft lighting that evokes a relaxing afternoon atmosphere."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1913759515700285569",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/56",
    "attributionUrl": "/creation/inspiration/attribution/case-56.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-56.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-55",
    "category": "product",
    "title": {
      "zh": "创意绿植花盆",
      "en": "Cute Plant Planter"
    },
    "prompt": {
      "zh": "一张高质量的照片，展示一个可爱的陶瓷[物体/动物]形状的花盆，表面光滑，里面装满了各种生机勃勃的多肉植物和绿色植物，包括尖刺的十二卷、莲座状的石莲花和精致的白色小花。花盆带有一个友好的面孔，放置在柔和的中性背景上，采用漫射自然光照明，展示了细腻的纹理和色彩对比，构图简洁、极具简约风格。",
      "en": "A high-quality photo of a cute ceramic [object/animal]-shaped planter with a glossy finish, filled with a variety of vibrant succulents and greenery including a spiky Haworthia, a rosette-shaped Echeveria, and delicate white flowers. The planter has a friendly face and sits on a soft, neutral background with diffused natural lighting, showcasing fine textures and color contrast in a clean, minimalistic composition"
    },
    "promptAuthor": "@azed_ai",
    "promptAuthorUrl": "https://x.com/azed_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/azed_ai/status/1923739813414568075",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/55",
    "attributionUrl": "/creation/inspiration/attribution/case-55.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-55.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-54",
    "category": "photography",
    "title": {
      "zh": "“极其平凡”的iPhone自拍",
      "en": "Extremely Ordinary iPhone Selfie"
    },
    "prompt": {
      "zh": "请画一张极其平凡无奇的iPhone 自拍照，没有明确的主体或构图感，就像是随手一拍的快照。照片略带运动模糊，阳光或店内灯光不均导致轻微曝光过度。角度尴尬、构图混乱，整体呈现出一种刻意的平庸感-就像是从口袋里拿手机时不小心拍到的一张自拍。主角是陈奕迅和谢霆锋，晚上，旁边是香港会展中心，在香港维多利亚港旁边。",
      "en": "Please draw an extremely ordinary and unremarkable iPhone selfie, with no clear subject or sense of composition — just like a random snapshot taken casually. The photo should include slight motion blur, with uneven lighting caused by sunlight or indoor lights resulting in mild overexposure. The angle is awkward, the composition is messy, and the overall aesthetic is deliberately plain — as if it was accidentally taken while pulling the phone out of a pocket.\nThe subjects are Eason Chan and Nicholas Tse, taken at night, next to the Hong Kong Convention and Exhibition Centre, by Victoria Harbour in Hong Kong."
    },
    "promptAuthor": "@jiamimaodashu",
    "promptAuthorUrl": "https://x.com/jiamimaodashu",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/jiamimaodashu/status/1912653073190879410",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/54",
    "attributionUrl": "/creation/inspiration/attribution/case-54.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-54.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-53",
    "category": "product",
    "title": {
      "zh": "Emoji 充气感靠垫",
      "en": "Emoji Inflatable Cushion"
    },
    "prompt": {
      "zh": "创建一个高分辨率的 3D 渲染图，将 [🥹] 设计成一个充气、鼓胀的物体。形状应柔软、圆润、充满空气——类似于一个毛绒气球或充气玩具。使用光滑的哑光材质，带有细微的布料折痕和缝线，以强化充气效果。整体形态应略带不规则且柔软塌陷，搭配柔和阴影和软光照，以突出体积感与真实感。将其置于干净、简约的背景上（浅灰色或浅蓝色），整体风格应保持俏皮而具雕塑感。",
      "en": "Create a high-resolution 3D render of [🥹] designed as an inflatable, puffy object. The shape should appear soft, rounded, and air-filled — like a plush balloon or blow-up toy. Use a smooth, matte texture with subtle fabric creases and stitching to emphasize the inflatable look. The form should be slightly irregular and squishy, with gentle shadows and soft lighting that highlight volume and realism. Place it on a clean, minimal background (light gray or pale blue), and maintain a playful, sculptural aesthetic."
    },
    "promptAuthor": "@gizakdag",
    "promptAuthorUrl": "https://x.com/gizakdag",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/gizakdag/status/1912858535643197927",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/53",
    "attributionUrl": "/creation/inspiration/attribution/case-53.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-53.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-52",
    "category": "illustration",
    "title": {
      "zh": "纸艺风格 Emoji 图标",
      "en": "Paper Craft Style Emoji Icon"
    },
    "prompt": {
      "zh": "一个纸艺风格的“🔥”图标，漂浮在纯白背景上。这个表情符号由彩色剪纸手工制作而成，具有可见的纸张纹理、折痕和分层形状。它在下方投下柔和的阴影，营造出轻盈感和立体感。整体设计简洁、有趣、干净，图像居中，周围留有大量留白。使用柔和的影棚光照以突出纸张的质感与边缘。",
      "en": "A paper craft-style \"🔥\" floating on a pure white background. The emoji is handcrafted from colorful cut paper with visible textures, creases, and layered shapes. It casts a soft drop shadow beneath, giving a sense of lightness and depth. The design is minimal, playful, and clean — centered in the frame with lots of negative space. Use soft studio lighting to highlight the paper texture and edges."
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1912521263085482464",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/52",
    "attributionUrl": "/creation/inspiration/attribution/case-52.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-52.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-51",
    "category": "poster",
    "title": {
      "zh": "护照入境印章",
      "en": "Passport Entry Stamp"
    },
    "prompt": {
      "zh": "创建一个逼真的护照页，并盖上[北京, 中国]的入境章。章面应以粗体英文写明“欢迎来到北京”，并设计成圆形或椭圆形，并带有装饰性边框。章面应包含“ARRIVAL”字样和一个虚构的日期，例如“2025年4月16日”。在章面中加入{故宫}的微妙轮廓作为背景细节。使用深蓝色或红色墨水并略加晕染，以增强真实感。章面应略微倾斜，如同手工压印。护照页应清晰可见纸张纹理和安全图案。",
      "en": "Create a realistic passport page with an entry stamp for [{City}, {Country}]. The stamp should say \"Welcome to {City}\" in bold English, designed in a round or oval shape with decorative borders. Include the word \"ARRIVAL\" and a fictional date like \"15 APR 2025\" Incorporate a subtle silhouette of {Main Landmark} as a background detail within the stamp. Use deep blue or red ink with light smudges for added realism. The stamp should appear slightly angled, as if hand-pressed. The passport page should show visible paper texture and security patterns."
    },
    "promptAuthor": "@M_w14_",
    "promptAuthorUrl": "https://x.com/M_w14_",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/M_w14_/status/1912146666410459618",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/51",
    "attributionUrl": "/creation/inspiration/attribution/case-51.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-51.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-50",
    "category": "illustration",
    "title": {
      "zh": "物理破坏效果卡片 (劳拉)",
      "en": "Physical Destruction Effect Card (Lara Croft)"
    },
    "prompt": {
      "zh": "一幅超写实、电影感的插画，描绘了劳拉·克劳馥动态地撞穿一张“考古探险”集换卡牌的边框。她正处于跳跃中或用绳索摆荡，穿着标志性的冒险装备，可能正在使用双枪射击，枪口的火焰帮助将卡牌古老的石雕边框震碎，在破口周围制造出可见的维度破裂效果，如能量裂纹和空间扭曲，使灰尘和碎片四散飞溅。她的身体充满活力地向前冲出，带有明显的运动深度，突破了卡牌的平面，卡牌内部（背景）描绘着茂密的丛林遗迹或布满陷阱的古墓内部。卡牌的碎屑与 crumbling 的石头、飞舞的藤蔓、古钱币碎片和用过的弹壳混合在一起。“考古探险”的标题和“劳拉·克劳馥”的名字（带有一个风格化的文物图标）在卡牌剩余的、布满裂纹和风化痕迹的部分上可见。充满冒险感的、动态的灯光突出了她的运动能力和危险的环境。",
      "en": "An ultra-photorealistic, cinematic-style illustration depicting Lara Croft dynamically bursting through the frame of an “Archaeological Adventure” trading card. She is caught mid-jump or swinging on a rope, wearing her iconic adventurer outfit and possibly firing dual pistols. The muzzle flashes help shatter the card’s ancient stone-carved border, creating a visible dimensional rupture with energy cracks and spatial distortions, scattering dust and debris outward.\n\nHer body lunges forward with powerful momentum, breaking through the card’s flat plane, emphasizing strong motion depth. Inside the card (the background) is a depiction of dense jungle ruins or a trap-filled ancient tomb. The shattered card fragments mix with crumbling stone, flying vines, broken ancient coins, and spent shell casings.\n\nThe title “Archaeological Adventure” and the name “Lara Croft” (accompanied by a stylized artifact icon) remain visible on the remaining cracked and weathered parts of the card. The scene is lit with adventurous, dynamic lighting that emphasizes her agility and the perilous environment."
    },
    "promptAuthor": "@op7418",
    "promptAuthorUrl": "https://x.com/op7418",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/op7418/status/1912782048160542886",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/50",
    "attributionUrl": "/creation/inspiration/attribution/case-50.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-50.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-49",
    "category": "poster",
    "title": {
      "zh": "时尚杂志封面风格",
      "en": "Fashion Magazine Cover Style"
    },
    "prompt": {
      "zh": "一位美丽的女子身穿粉色旗袍，头戴精致的花饰，秀发中点缀着色彩缤纷的花朵，颈间装饰着优雅的白色蕾丝领子。她的一只手轻托着几只大型蝴蝶。整体拍摄风格呈现高清细节质感，类似时尚杂志封面设计，照片上方中央位置标有文字「FASHION DESIGN」。画面背景采用简约的纯浅灰色，以突出人物主体。",
      "en": "A beautiful woman wearing a pink qipao, adorned with delicate floral accessories on her head and colorful blossoms woven into her hair. Around her neck is an elegant white lace collar. One of her hands gently holds several large butterflies. The overall photography style features high-definition detail and texture, resembling a fashion magazine cover. The word “FASHION DESIGN” is placed at the top center of the image. The background is a minimalist light gray, designed to highlight the subject."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1912536019905233194",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/49",
    "attributionUrl": "/creation/inspiration/attribution/case-49.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-49.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-48",
    "category": "illustration",
    "title": {
      "zh": "体素风格 3D 图标转换",
      "en": "Voxel Style 3D Icon Conversion"
    },
    "prompt": {
      "zh": "三个步骤\n1. 上传参考图\n2. 上传要转换的照片\n3. 提示词：将图片/描述/emoji转换为参考图一样的体素 3D 图标，Octane 渲染，8k",
      "en": "Take the icon on the right and transform it into a voxel 3d icon like the icons in the left image. Octane render. 8k."
    },
    "promptAuthor": "@BrettFromDJ",
    "promptAuthorUrl": "https://x.com/BrettFromDJ",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/BrettFromDJ/status/1910387413404234076",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/48",
    "attributionUrl": "/creation/inspiration/attribution/case-48.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-48.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-47",
    "category": "product",
    "title": {
      "zh": "键盘ESC 键帽微型立体模型",
      "en": "ESC Keycap Miniature Diorama"
    },
    "prompt": {
      "zh": "一个超写实的等距视角 3D 渲染图，展示了一个微型电脑工作空间，置于一个半透明的机械键盘键帽内，键帽特别放置在一块真实哑光表面的机械键盘的 ESC 键上。\n键帽内部，一个穿着舒适、有纹理连帽衫的小人坐在现代人体工学椅上，正专注地面对一块发光的超写实电脑屏幕工作。整个空间布满了逼真的微型科技配件：真实材质的台灯、带有反射效果的显示器、微小的扬声器格栅、缠绕的电缆以及陶瓷杯子。\n场景底部由土壤、岩石和苔藓构成，拥有照片级的材质质感和自然瑕疵。键帽内的光照模拟清晨自然阳光，投下柔和阴影与温暖光调；而键帽外部则受周围键盘环境的冷色调反射影响。\n“ESC”字样以微弱的磨砂玻璃效果蚀刻在半透明键帽顶部——根据视角不同，仅隐约可见。\n周围的按键如 F1、Q、Shift 和 CTRL 均清晰可见，拥有真实材质纹理与光照。整体画面仿佛由高端手机相机拍摄，具备浅景深、完美白平衡与电影感细节。",
      "en": "A hyper-realistic isometric 3D render of a miniature computer setup inside a translucent mechanical keyboard keycap, specifically placed on the ESC key of a real matte-finished mechanical keyboard. Inside the keycap, a tiny figure sits in a modern ergonomic chair, wearing a cozy textured hoodie, working at a glowing ultra-realistic computer screen. The environment is packed with lifelike miniature tech accessories: real-material desk lamps, monitors with reflections, tiny speaker grills, tangled cables, and ceramic mugs. The base of the scene is made of soil, rocks, and moss, with photorealistic textures and imperfections. The lighting inside the cap mimics natural morning sun, casting soft shadows and warm tones, while the outside has cold ambient reflections from the surrounding keyboard. The word “ESC” is subtly etched onto the top of the translucent keycap with a faint frosted glass effect — just barely visible depending on the angle. The surrounding keyboard keys like F1, Q, Shift, and CTRL are crisp, textured, and photorealistically lit. Shot as if taken with a high-end mobile phone camera, with shallow depth of field, perfect white balance, and cinematic detail."
    },
    "promptAuthor": "@egeberkina",
    "promptAuthorUrl": "https://x.com/egeberkina",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/egeberkina/status/1911368319212408926",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/47",
    "attributionUrl": "/creation/inspiration/attribution/case-47.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-47.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-46",
    "category": "product",
    "title": {
      "zh": "快乐胶囊制作",
      "en": "Happy Capsule Creation"
    },
    "prompt": {
      "zh": "标题（大字）：速效快乐胶囊\n一颗上为星巴克绿下为透明的小药丸，上面印有星巴克logo，里面有很多咖啡豆\n说明（小字）：请在悲伤难过时服用，一日三次，一次两粒\n购买按钮 和 药丸颜色一致，下面价格：$9，请遵循医嘱酌情购买",
      "en": "Title (large text): Fast-Acting Happiness Capsule\nA capsule pill with a green top in Starbucks green and a transparent bottom, printed with the Starbucks logo. Inside the capsule are numerous coffee beans.\nDescription (small text): Take when feeling sad or down. Three times a day, two capsules per dose.\nBuy button: Same color as the capsule.\nBelow the button, display the price: $9.\nPlease follow medical advice and purchase as needed."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1911724629460455896",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/46",
    "attributionUrl": "/creation/inspiration/attribution/case-46.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-46.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-45",
    "category": "illustration",
    "title": {
      "zh": "3D Q版大学拟人化形象",
      "en": "3D Chibi-style University Anthropomorphic Mascot"
    },
    "prompt": {
      "zh": "给 {西北工业大学} 画一个拟人化的3D Q版美少女形象，体现学校 {航空航天航海三航} 特色",
      "en": "Create a personified 3D chibi-style anime girl character representing {Northwestern Polytechnical University}, embodying the school’s distinctive strengths in {aeronautics, astronautics, and marine engineering}."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1911988003729203648",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/45",
    "attributionUrl": "/creation/inspiration/attribution/case-45.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-45.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-44",
    "category": "poster",
    "title": {
      "zh": "RPG 风格角色卡片制作",
      "en": "RPG-Style Character Card Creation"
    },
    "prompt": {
      "zh": "创建一张 RPG 收藏风格的数字角色卡。\n角色设定为 {Programmer}，自信地站立，配有与其职业相关的工具或符号。\n以 3D 卡通风格呈现，采用柔和光照，展现鲜明的个性。\n添加技能条或属性数值，例如 [技能1 +x]、[技能2 +x]，如 Creativity +10、UI/UX +8。\n卡片顶部添加标题横幅，底部放置角色名牌。\n卡片边框应干净利落，如同真实的收藏公仔包装盒。\n背景需与职业主题相匹配。\n配色方面使用温暖的高光与符合职业特征的色调。",
      "en": "Create a digital character card in RPG collectible style.\nThe subject is a {Programmer}, standing confidently with tools or symbols relevant to their job.\nRender it in 3D cartoon style, soft lighting, vivid personality.\nInclude skill bars or stats like [Skill1 +x], [Skill2 +x, e.g., Creativity +10, UI/UX +8].\nAdd a title banner on top and a nameplate on the bottom.\nFrame the card with clean edges like a real figure box.\nMake the background fit the profession's theme.\nColors: warm highlights, profession-matching hues."
    },
    "promptAuthor": "@berryxia_ai",
    "promptAuthorUrl": "https://x.com/berryxia_ai",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/berryxia_ai/status/1911334046724165905",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/44",
    "attributionUrl": "/creation/inspiration/attribution/case-44.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-44.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-43",
    "category": "product",
    "title": {
      "zh": "Q版可爱俄罗斯套娃 (戴珍珠耳环的少女)",
      "en": "Cute Chibi Matryoshka Dolls (Girl with a Pearl Earring)"
    },
    "prompt": {
      "zh": "把图片人物生成变成 Q 版可爱俄罗斯套娃🪆，大到小一共五个，放在精致的木桌上，横幅3:2比例",
      "en": "Transform the person in the image into a set of cute chibi-style Russian nesting dolls (🪆), with a total of five dolls arranged from largest to smallest. Place them on an elegant wooden table. Horizontal aspect ratio: 3:2."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1911669883315818497",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/43",
    "attributionUrl": "/creation/inspiration/attribution/case-43.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-43.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-42",
    "category": "product",
    "title": {
      "zh": "3D Q版情侣水晶球",
      "en": "3D Q-version Couple Snow Globe"
    },
    "prompt": {
      "zh": "将附图中的人物转换成水晶球场景。 整体环境：水晶球放在窗户旁桌面上，背景模糊，暖色调。阳光透过球体，洒下点点金光，照亮了周围的黑暗。 水晶球内部：人物是可爱Q版3D造型，相互之间满眼的爱意。",
      "en": "Transform the person in the attached image into a snow globe scene.\nOverall environment: The snow globe is placed on a tabletop by the window, with a blurred, warm-toned background. Sunlight passes through the globe, casting golden sparkles that gently illuminate the surrounding darkness.\nInside the globe: The characters are in a cute chibi-style 3D design, gazing at each other with eyes full of love."
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1909908568129655248",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/42",
    "attributionUrl": "/creation/inspiration/attribution/case-42.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-42.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-41",
    "category": "architecture",
    "title": {
      "zh": "微型立体场景 (孙悟空三打白骨精)",
      "en": "Miniature Three-dimensional Scene Presentation"
    },
    "prompt": {
      "zh": "微型立体场景呈现，运用移轴摄影的技法，呈现出Q版【孙悟空三打白骨精】场景",
      "en": "Miniature three-dimensional scene presentation using tilt-shift photography techniques, depicting a chibi-style version of the scene {Sun Wukong’s Three Battles with the White Bone Demon}"
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1911609122547449886",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/41",
    "attributionUrl": "/creation/inspiration/attribution/case-41.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-41.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-40",
    "category": "illustration",
    "title": {
      "zh": "日系双格漫画 (少女总统红温了)",
      "en": "Japanese-style Two-Panel Manga (Angry Girl President)"
    },
    "prompt": {
      "zh": "创建一张日系萌系双格漫画，上下排列，主题：少女总统的工作日常。\n\n角色形象: 将上传的附件转换为日系萌系卡通女生形象的风格，保留原图所有细节，如服饰（西装）、发型（明亮的金黄色）、五官等。 \n\n第一格: \n- 表情: 委屈巴巴，沮丧的表情，单手托腮 \n- 文字框: “肿么办嘛！他不跟我通话！(；´д｀)” \n- 场景: 暖色调办公室，背后美国国旗，桌上放着一堆汉堡，一个复古红色转盘电话，人物在画面左边，电话在右边。  \n\n第二格:  \n- 表情: 咬牙切齿，暴怒，脸涨红 \n- 动作: 猛拍桌子，汉堡震得跳起来 \n- 文字泡: “哼！关税加倍！不理我是他们的损失！( `д´ )” - 场景: 和第一格相同，但一片狼藉。  \n\n其他说明:  \n- 文字采用简洁可爱的手写体，整体风格可爱而有趣。 \n- 构图饱满生动，请保留足够空间用于文字显示，适当留白。 \n- 图片比例 2:3。 \n- 画面整体色彩鲜艳，突出卡通风格。",
      "en": "Create a two-panel vertical manga in a cute Japanese anime style, theme: “The Daily Work Life of a Girl President.”\n\nCharacter Design:\nTransform the person in the uploaded image into a cute, moe-style anime girl while preserving all key details from the photo — including the outfit (a suit), hairstyle (bright golden-yellow), and facial features.\n\nPanel 1:\n- Expression: Pouting, disappointed, resting her cheek on one hand\n- Text box: “What do I dooo?! He won’t take my call! (；´д｀)”\n- Scene: Warm-toned office, with the U.S. flag in the background. On the desk: a pile of hamburgers and a vintage red rotary phone. The character is on the left side of the frame, the phone on the right.\n\nPanel 2:\n- Expression: Furious, face red with anger, gritting teeth\n- Action: Slams the desk hard, making the hamburgers jump\n- Speech bubble: “Hmph! Double the tariffs! Ignoring me is their loss! ( `д´ )”\n- Scene: Same office, now a complete mess\n\nAdditional Notes:\n- Use a cute, casual handwritten font for all text\n- Keep the composition full and expressive, with adequate space for dialogue and intentional white space\n- Aspect ratio: 2:3\n- The overall visual tone should be colorful and energetic, with a distinctly cartoony style"
    },
    "promptAuthor": "@hellokaton",
    "promptAuthorUrl": "https://x.com/hellokaton",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/hellokaton/status/1910900979194646959",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/40",
    "attributionUrl": "/creation/inspiration/attribution/case-40.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-40.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-38",
    "category": "poster",
    "title": {
      "zh": "手绘信息图卡片",
      "en": "Hand-drawn Infographic Card"
    },
    "prompt": {
      "zh": "创作一张手绘风格的信息图卡片，比例为9:16竖版。卡片主题鲜明，背景为带有纸质肌理的米色或米白色，整体设计体现质朴、亲切的手绘美感。\n\n卡片上方以红黑相间、对比鲜明的大号毛笔草书字体突出标题，吸引视觉焦点。文字内容均采用中文草书，整体布局分为2至4个清晰的小节，每节以简短、精炼的中文短语表达核心要点。字体保持草书流畅的韵律感，既清晰可读又富有艺术气息。周边适当留白。\n\n卡片中点缀简单、有趣的手绘插画或图标，例如人物或象征符号，以增强视觉吸引力，引发读者思考与共鸣。整体布局注意视觉平衡，预留足够的空白空间，确保画面简洁明了，易于阅读和理解。\n“做 IP 是长期复利\n坚持每日更新，肯定会有结果，因为 99% 都坚持不了的！”",
      "en": "Create a hand-drawn style infographic card in a 9:16 vertical format. The card should have a clear theme, with a beige or off-white paper-textured background. The overall design should reflect a simple, warm, and handmade aesthetic.\n\nAt the top of the card, use large, eye-catching brush-style Chinese cursive calligraphy in red and black for the title, creating strong visual contrast. All text should be in Chinese cursive script. The layout should be divided into 2 to 4 clear sections, each conveying a core idea through concise and refined Chinese phrases. The calligraphy should maintain a fluid, rhythmic style that is both legible and artistically expressive. Leave appropriate blank space around the text.\n\nThe card should be accented with simple and fun hand-drawn illustrations or icons — such as figures or symbolic elements — to enhance visual appeal and spark thought or emotional resonance. The overall layout should emphasize visual balance and include ample whitespace, ensuring the design is clean, clear, and easy to read.\n\n“Building a personal brand (IP) is long-term compounding.\nKeep updating daily, and results will come — because 99% of people can’t keep it up!”"
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1907870919852179850",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/38",
    "attributionUrl": "/creation/inspiration/attribution/case-38.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-38.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-37",
    "category": "poster",
    "title": {
      "zh": "柔和风格3D广告",
      "en": "Pastel Power 3D ADS"
    },
    "prompt": {
      "zh": "一个柔和的3D卡通风格[品牌产品]雕塑，由光滑的粘土般纹理和鲜艳的柔和色彩制成，放置在简约的等距场景中，该场景与产品特性相得益彰，构图简洁，光线柔和，阴影微妙，产品徽标和三个词的口号清晰显示在下方。",
      "en": "a soft 3D cartoon-style sculpture of [brand product], made of smooth clay-like textures and vibrant pastel colors, placed in a minimalist isometric scene that complements the product’s nature, clean composition, gentle lighting, subtle shadows, with the product’s logo and a 3-word slogan displayed clearly below"
    },
    "promptAuthor": "@op7418",
    "promptAuthorUrl": "https://x.com/op7418",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/op7418/status/1906208691877253536",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/37",
    "attributionUrl": "/creation/inspiration/attribution/case-37.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-37.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-36",
    "category": "illustration",
    "title": {
      "zh": "极简主义 3D 插画 (Markdown 格式)",
      "en": "Minimalist 3D Illustration (Markdown Format)"
    },
    "prompt": {
      "zh": "画一个马桶：\n\n## 艺术风格简介：极简主义3D插画（Minimalist 3D Illustration）\n\n### 🎨 视觉元素（Visual Elements）\n\n#### 🟢 造型语言（Shape Language）\n- 圆润的边缘、平滑柔和的外形，采用简化几何造型。\n\n#### 🎨 色彩（Colors）\n- **主色调：** 柔和米色、浅灰色、暖橙色。\n- **强调色：** 暖橙色用于焦点元素。\n- **明暗处理：** 柔和渐变，平滑过渡，避免强烈的阴影和高光。\n\n#### 💡 光照（Lighting）\n- **类型：** 柔和、漫反射光照。\n- **光源方向：** 上方稍偏右。\n- **阴影风格：** 微妙且漫射，无锐利或高对比度的阴影。\n\n#### 🧱 材质（Materials）\n- **表面纹理：** 哑光、平滑的表面，带有微妙的明暗变化。\n- **反射性：** 低或无，避免明显的光泽。\n\n#### 🖼️ 构图（Composition）\n- **对象呈现：** 单一、居中的物体，周围留出大量负空间。\n- **视角：** 轻微倾斜视角，呈现适度的三维感，但无明显的景深效果。\n- **背景：** 纯色、低饱和度，与主体协调且不干扰视线。\n\n#### ✒️ 字体排版（Typography）\n- **字体风格：** 极简、无衬线字体。\n- **文字位置：** 左下角，尺寸小巧且不突出。\n- **字体颜色：** 灰色，与背景形成低对比度。\n\n#### 🖥️ 渲染风格（Rendering Style）\n- **技术手法：** 3D渲染，采用简化的低多边形风格。\n- **细节程度：** 中等细节，以形状和色彩为主，避免复杂纹理和细节。\n\n### 🎯 风格目标（Purpose）\n> 创建干净、美观的视觉效果，强调简洁、亲和和现代感。",
      "en": "Draw a Toilet\n\n## 🎨 Art Style: Minimalist 3D Illustration\n\n### 🟢 Shape Language\n- Rounded edges and smooth, soft forms using simplified geometric shapes.\n\n### 🎨 Colors\n- **Primary palette:** soft beige, light gray, warm orange.  \n- **Accent color:** warm orange for focal elements.  \n- **Shading:** gentle gradients and smooth transitions, avoiding harsh shadows and highlights.\n\n### 💡 Lighting\n- **Type:** soft, diffuse lighting.  \n- **Light source direction:** from above, slightly to the right.  \n- **Shadow style:** subtle and diffused, without sharp or high-contrast shadows.\n\n### 🧱 Materials\n- **Surface texture:** matte and smooth with subtle light variation.  \n- **Reflectivity:** low to none, avoiding noticeable gloss.\n\n### 🖼️ Composition\n- **Object presentation:** a single, centered object with generous negative space around it.  \n- **Perspective:** slight tilt to suggest depth, but no strong depth-of-field effects.  \n- **Background:** flat color, low saturation, harmonious with the subject and non-distracting.\n\n### ✒️ Typography\n- **Font style:** minimalist sans-serif.  \n- **Text placement:** bottom left corner, small and unobtrusive.  \n- **Font color:** gray, low contrast with the background.\n\n### 🖥️ Rendering Style\n- **Technique:** 3D rendering in a simplified low-poly style.  \n- **Detail level:** medium — focus on shape and color, avoiding complex textures or fine details.\n\n## 🎯 Style Goal\n> Create a clean and aesthetically pleasing visual that emphasizes simplicity, approachability, and modernity."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1907131027253772399",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/36",
    "attributionUrl": "/creation/inspiration/attribution/case-36.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-36.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-35",
    "category": "product",
    "title": {
      "zh": "毛茸茸南瓜灯",
      "en": "Fluffy Jack-o'-lantern"
    },
    "prompt": {
      "zh": "将一个简单平面的矢量图标 [🎃] 转化为柔软、立体、毛茸茸的可爱物体。整体造型被浓密的毛发完全覆盖，毛发质感极其真实，带有柔和的阴影。物体居中悬浮于干净的浅灰色背景中，轻盈漂浮。整体风格超现实，富有触感和现代感，带来舒适和俏皮的视觉感受。采用摄影棚级灯光，高分辨率渲染，比例为1:1。",
      "en": "Transform a simple flat vector icon of [🎃] into a soft, 3D fluffy object. The shape is fully covered in fur, with hyperrealistic hair texture and soft shadows. The object is centered on a clean, light gray background and floats gently in space. The style is surreal, tactile, and modern, evoking a sense of comfort and playfulness. Studio lighting, high-resolution render."
    },
    "promptAuthor": "gizakdag",
    "promptAuthorUrl": "https://x.com/gizakdag",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/gizakdag/status/1911075302941622512",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/35",
    "attributionUrl": "/creation/inspiration/attribution/case-35.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-35.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-34",
    "category": "poster",
    "title": {
      "zh": "手绘信息图卡片",
      "en": "Hand-drawn Infographic Card (Cognition)"
    },
    "prompt": {
      "zh": "创作一张手绘风格的信息图卡片，比例为9:16竖版。卡片主题鲜明，背景为带有纸质肌理的米色或米白色，整体设计体现质朴、亲切的手绘美感。\n\n卡片上方以红黑相间、对比鲜明的大号毛笔草书字体突出标题，吸引视觉焦点。文字内容均采用中文草书，整体布局分为2至4个清晰的小节，每节以简短、精炼的中文短语表达核心要点。字体保持草书流畅的韵律感，既清晰可读又富有艺术气息。\n\n卡片中点缀简单、有趣的手绘插画或图标，例如人物或象征符号，以增强视觉吸引力，引发读者思考与共鸣。\n整体布局注意视觉平衡，预留足够的空白空间，确保画面简洁明了，易于阅读和理解。\n\n<h1><span style=\"color:red\">「认知」</span>决定上限\n<span style=\"color:red\">「圈子」</span>决定机会</h1>\n- 你赚不到「认知」以外的钱，\n- 也遇不到「圈子」以外的机会。",
      "en": "Create a hand-drawn style infographic card in vertical 9:16 ratio. The card should have a clear theme, with a beige or off-white paper-textured background. The overall design should convey a rustic, friendly, and handmade aesthetic.\n\nAt the top of the card, feature a bold, eye-catching title in large Chinese cursive brush calligraphy using contrasting red and black colors. All text content should be in Chinese cursive script, and the layout should be divided into 2 to 4 clear sections. Each section expresses a core idea with brief and concise Chinese phrases. The cursive font should retain a smooth, rhythmic flow, remaining legible while carrying artistic appeal.\n\nThe card should include simple, playful hand-drawn illustrations or icons, such as figures or symbolic elements, to enhance visual interest and spark reader reflection or emotional resonance.\n\nThe overall layout should maintain visual balance, with ample white space reserved to ensure clarity, simplicity, and ease of reading and understanding.\n<h1><span style=\"color:red\">“Cognition”</span> defines your ceiling  \n<span style=\"color:red\">“Circle”</span> defines your opportunities</h1>  \n– You can’t earn money beyond your level of cognition,  \n– Nor encounter opportunities beyond your social circle."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1907903480678985784",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/34",
    "attributionUrl": "/creation/inspiration/attribution/case-34.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-34.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-33",
    "category": "illustration",
    "title": {
      "zh": "全家福婚纱照",
      "en": "Family Wedding Photo (Q-version)"
    },
    "prompt": {
      "zh": "将照片里的转换成Q版 3D人物，父母婚礼服饰，孩子是美丽的花童。 父母，西式婚礼服饰，父亲礼服，母亲婚纱。孩子手捧鲜花。 背景是五彩鲜花做的拱门。 除了人物是3D Q版，环境其他都是写实。整体放在一个相框里。",
      "en": "Transform the people in the photo into chibi-style 3D characters. The parents are dressed in Western wedding attire — the father in a formal suit, the mother in a wedding gown. The child is a beautiful flower girl holding a bouquet.\n\nThe background features a colorful floral arch.\nThe characters are in 3D chibi style, while the environment is photorealistic.\nThe entire scene is placed inside a photo frame."
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1909426314643222595",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/33",
    "attributionUrl": "/creation/inspiration/attribution/case-33.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-33.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-31",
    "category": "illustration",
    "title": {
      "zh": "动漫贴纸集合",
      "en": "Anime Sticker Collection"
    },
    "prompt": {
      "zh": "火影忍者贴纸",
      "en": "Naruto stickers"
    },
    "promptAuthor": "@richardchang",
    "promptAuthorUrl": "https://x.com/richardchang",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/richardchang/status/1909086122959139312",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/31",
    "attributionUrl": "/creation/inspiration/attribution/case-31.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-31.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-29",
    "category": "photography",
    "title": {
      "zh": "名画人物 OOTD",
      "en": "Famous Painting Character OOTD"
    },
    "prompt": {
      "zh": "为图片人物生成不同职业风的OOTD，时尚穿搭和配饰，和人物色系一致的纯色背景，Q版 3d，c4d渲染，保持人脸特征，姿势都要保持一致，人物的比例腿很修长\n\n构图：9:16\n顶部文字：OOTD，左侧为人物ootd q版形象，右侧为穿搭的单件展示\n\n先来第一个职业：时尚设计师",
      "en": "Generate a Q-style 3D C4D-rendered character based on the person in the photo, dressed in a fashion-forward “outfit of the day” (OOTD) inspired by a specific profession.\nProfession: Fashion Designer\n– Keep the original facial features and character pose\n– Stylize the character with a cute, long-legged chibi proportion\n– Outfit and accessories should reflect the profession, including trendy designer wear, glasses, sketchbook or tablet, and stylish shoes\n– Match the outfit with fashion accessories to complete the look\n– Use a solid background color that complements the character’s overall color palette (no gradients or textures)\n\nComposition: Aspect ratio: 9:16\nTop text: “OOTD”\nLeft side: the full-body chibi character wearing the complete outfit\nRight side: individual clothing items and accessories laid out separately, as if in a style breakdown"
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1909892294217781714",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/29",
    "attributionUrl": "/creation/inspiration/attribution/case-29.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-29.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-28",
    "category": "illustration",
    "title": {
      "zh": "扁平贴纸设计",
      "en": "Flat Sticker Design"
    },
    "prompt": {
      "zh": "把这张照片设计成一个极简扁平插画风格的Q版贴纸，厚白边，保留人物特征，风格要可爱一些，人物要超出圆形区域边框，圆形区域要为纯色不要3d感，透明背景。",
      "en": "Turn this photo into a chibi-style sticker illustration in a minimalist flat design.\n– Keep the character’s recognizable features\n– Use a cute, simplified aesthetic\n– The sticker should have a thick white border\n– The character should break out of the circular frame, adding a playful touch\n– The circular base should be a solid flat color (no 3D or gradients)\n– Background should be transparent\nThe overall style should be clean, modern, and visually appealing for use as a fun Q-version sticker."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1908044836953108490",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/28",
    "attributionUrl": "/creation/inspiration/attribution/case-28.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-28.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-27",
    "category": "illustration",
    "title": {
      "zh": "Q 版表情包制作",
      "en": "Q-version Emoji Sticker Pack Creation"
    },
    "prompt": {
      "zh": "创作一套全新的 chibi sticker，共六个独特姿势，以用户形象为主角：\n1. 双手比出剪刀手，俏皮地眨眼；\n2. 泪眼汪汪、嘴唇微微颤动，呈现可爱哭泣的表情；\n3. 张开双臂，做出热情的大大拥抱姿势；\n4. 侧卧入睡，靠着迷你枕头，带着甜甜的微笑；\n5. 自信满满地向前方伸手指，周围点缀闪亮特效；\n6. 手势飞吻，周围飘散出爱心表情。\n保留 chibi 美学风格：夸张有神的大眼睛、柔和的面部线条、活泼俏皮的短款黑色发型、配以大胆领口设计的白色服饰，背景使用充满活力的红色，并搭配星星或彩色纸屑元素进行装饰。周边适当留白。\nAspect ratio: 9:16",
      "en": "Create a brand-new set of chibi-style stickers featuring the user as the main character, with six unique poses:\n  1.\tMaking a playful peace sign with both hands and winking.\n  2.\tTearful eyes and slightly trembling lips, showing a cute crying expression.\n  3.\tArms wide open in a warm, enthusiastic hug pose.\n  4.\tLying on their side asleep, resting on a tiny pillow with a sweet smile.\n  5.\tPointing forward with confidence, surrounded by shining visual effects.\n  6.\tBlowing a kiss, with heart symbols floating around.\nMaintain the chibi aesthetic:\n– Exaggerated, expressive big eyes\n– Soft facial lines\n– Playful, short black hairstyle\n– A white outfit with a bold neckline design\nBackground: Vibrant red with star or colorful confetti elements for decoration. Leave some clean white space around each sticker.\nAspect ratio: 9:16"
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1909800530739679488",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/27",
    "attributionUrl": "/creation/inspiration/attribution/case-27.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-27.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-26",
    "category": "poster",
    "title": {
      "zh": "名画人物麦片广告",
      "en": "Famous Painting Character Cereal Ad"
    },
    "prompt": {
      "zh": "《大师麦片》：根据我上传的照片的人物特征判断，为他生成一个符合他特质的燕麦片搭配（比如蔬菜、水果、酸奶、粗粮等等）和包装设计，然后生成他作为麦片包装盒封面人物 加 相应麦片搭配的广告封面，人物要保持特征、可爱Q版3d、c4d渲染风格，麦片所放置的地方的风格也要符合设定，比如放在厨房、超市 极简主义的设计台上等等，先做好设定，再生成图像。",
      "en": "“Master Oats”: Based on the visual features of the person in the uploaded photo, generate a custom oatmeal mix that reflects their personality traits — for example, using vegetables, fruits, yogurt, whole grains, etc.\n\nDesign a unique cereal box and package aesthetic that aligns with this tailored mix.\n\nThen, create an advertising cover featuring the person as the mascot on the cereal box. The character should retain their recognizable features but be transformed into a cute chibi-style 3D figure with a C4D-quality rendering.\n\nThe oatmeal and packaging should be presented in a setting that matches the mood — such as a minimalist kitchen, a sleek supermarket display, or a clean design counter.\n\nThe process includes:\n– Character analysis and oat mix pairing\n– Cereal box concept and design\n– Display environment selection\n– Final image with mascot figure, packaging, and styled scene composition\n\nAll visuals should be balanced, modern, and appealing, reflecting a premium and fun oat brand identity."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1909542765857587310",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/26",
    "attributionUrl": "/creation/inspiration/attribution/case-26.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-26.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-25",
    "category": "illustration",
    "title": {
      "zh": "极简主义 3D 插画",
      "en": "Minimalist 3D Illustration"
    },
    "prompt": {
      "zh": "使用以下 JSON 配置文件生成一个马桶：\n{\n  \"art_style_profile\": {\n    \"style_name\": \"Minimalist 3D Illustration\",\n    \"visual_elements\": {\n      \"shape_language\": \"Rounded edges, smooth and soft forms with simplified geometry\",\n      \"colors\": {\n        \"primary_palette\": [\"Soft beige, light gray, warm orange\"],\n        \"accent_colors\": [\"Warm orange for focal elements\"],\n        \"shading\": \"Soft gradients with smooth transitions, avoiding harsh shadows or highlights\"\n      },\n      \"lighting\": {\n        \"type\": \"Soft, diffused lighting\",\n        \"source_direction\": \"Above and slightly to the right\",\n        \"shadow_style\": \"Subtle and diffused, no sharp or high-contrast shadows\"\n      },\n      \"materials\": {\n        \"surface_texture\": \"Matte, smooth surfaces with subtle shading\",\n        \"reflectivity\": \"Low to none, avoiding glossiness\"\n      },\n      \"composition\": {\n        \"object_presentation\": \"Single, central object displayed in isolation with ample negative space\",\n        \"perspective\": \"Slightly angled, giving a three-dimensional feel without extreme depth\",\n        \"background\": \"Solid, muted color that complements the object without distraction\"\n      },\n      \"typography\": {\n        \"font_style\": \"Minimalistic, sans-serif\",\n        \"text_placement\": \"Bottom-left corner with small, subtle text\",\n        \"color\": \"Gray, low-contrast against the background\"\n      },\n      \"rendering_style\": {\n        \"technique\": \"3D render with simplified, low-poly aesthetics\",\n        \"detail_level\": \"Medium detail, focusing on form and color over texture or intricacy\"\n      }\n    },\n    \"purpose\": \"To create clean, aesthetically pleasing visuals that emphasize simplicity, approachability, and modernity.\"\n  }\n}",
      "en": "Generate a toilet with the following JSON profile:\n{\n  \"art_style_profile\": {\n    \"style_name\": \"Minimalist 3D Illustration\",\n    \"visual_elements\": {\n      \"shape_language\": \"Rounded edges, smooth and soft forms with simplified geometry\",\n      \"colors\": {\n        \"primary_palette\": [\"Soft beige, light gray, warm orange\"],\n        \"accent_colors\": [\"Warm orange for focal elements\"],\n        \"shading\": \"Soft gradients with smooth transitions, avoiding harsh shadows or highlights\"\n      },\n      \"lighting\": {\n        \"type\": \"Soft, diffused lighting\",\n        \"source_direction\": \"Above and slightly to the right\",\n        \"shadow_style\": \"Subtle and diffused, no sharp or high-contrast shadows\"\n      },\n      \"materials\": {\n        \"surface_texture\": \"Matte, smooth surfaces with subtle shading\",\n        \"reflectivity\": \"Low to none, avoiding glossiness\"\n      },\n      \"composition\": {\n        \"object_presentation\": \"Single, central object displayed in isolation with ample negative space\",\n        \"perspective\": \"Slightly angled, giving a three-dimensional feel without extreme depth\",\n        \"background\": \"Solid, muted color that complements the object without distraction\"\n      },\n      \"typography\": {\n        \"font_style\": \"Minimalistic, sans-serif\",\n        \"text_placement\": \"Bottom-left corner with small, subtle text\",\n        \"color\": \"Gray, low-contrast against the background\"\n      },\n      \"rendering_style\": {\n        \"technique\": \"3D render with simplified, low-poly aesthetics\",\n        \"detail_level\": \"Medium detail, focusing on form and color over texture or intricacy\"\n      }\n    },\n    \"purpose\": \"To create clean, aesthetically pleasing visuals that emphasize simplicity, approachability, and modernity.\"\n  }\n}"
    },
    "promptAuthor": "@0xdlk",
    "promptAuthorUrl": "https://x.com/0xdlk",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/0xdlk/status/1906843247432929642",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/25",
    "attributionUrl": "/creation/inspiration/attribution/case-25.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-25.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-24",
    "category": "product",
    "title": {
      "zh": "Funko Pop 公仔制作",
      "en": "Funko Pop Figure Creation"
    },
    "prompt": {
      "zh": "把照片中的人物变成 Funko Pop 公仔包装盒的风格，以等距视角（isometric）呈现，并在包装盒上标注标题为“JAMES BOND”。包装盒内展示的是照片中人物形象，旁边搭配有人物的必备物品（手枪、手表、西装、其他）同时，在包装盒旁边还应呈现该公仔本体的实物效果，采用逼真的、具有真实感的渲染风格。",
      "en": "Transform the person in the photo into the style of a Funko Pop figure box, presented in isometric view.\nThe packaging is labeled with the title “JAMES BOND.”\nInside the box, display a chibi-style figure based on the person in the photo, along with their essential accessories: a pistol, a wristwatch, a suit, and other signature items.\nNext to the box, show a realistic rendering of the actual figure outside the packaging, with detailed textures and lighting to achieve a lifelike product display."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1909047283485671924",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/24",
    "attributionUrl": "/creation/inspiration/attribution/case-24.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-24.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-23",
    "category": "illustration",
    "title": {
      "zh": "《泰坦尼克号》模仿",
      "en": "\"Titanic\" Pose Parody"
    },
    "prompt": {
      "zh": "将附图中的人物转换成可爱Q版3D造型\n场景：在豪华游轮最顶尖的船头，船头是尖的。\n男士带着女士站在泰坦尼克号船头，男士双手搂着女士的腰，女士双臂伸展穿着连衣裙，迎着风，脸上洋溢着自由与畅快。\n此时天色呈现出黄昏的暖色调，大海在船下延展。\n除了人物用Q版3D造型以外，其他环境都是实物。",
      "en": "Transform the person in the attached image into a cute chibi-style 3D character.\nScene: On the pointed bow of a luxurious cruise ship.\nThe man stands behind the woman at the bow, holding her waist with both hands. The woman is wearing a dress, arms spread wide, facing the wind, with a joyful and liberated expression on her face—just like the iconic scene from Titanic.\nThe sky is painted in warm sunset tones, and the vast ocean stretches beneath the ship.\nOnly the characters should be in chibi 3D style; the rest of the environment should be realistic."
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1909916265067557299",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/23",
    "attributionUrl": "/creation/inspiration/attribution/case-23.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-23.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-22",
    "category": "poster",
    "title": {
      "zh": "小红书封面",
      "en": "Xiaohongshu Cover Image"
    },
    "prompt": {
      "zh": "画图：画一个小红书封面。\n要求：\n有足够的吸引力吸引用户点击；\n字体醒目，选择有个性的字体；\n文字大小按重要度分级，体现文案的逻辑结构；\n标题是普通文字的至少2倍；\n文字段落之间留白。\n只对要强调的文字用醒目色吸引用户注意；\n背景使用吸引眼球的图案（包括不限于纸张，记事本，微信聊天窗口，选择一种）\n使用合适的图标或图片增加视觉层次，但要减少干扰。\n\n文案：重磅！ChatGPT又变强了！\n多任务处理更牛✨\n编程能力更强💪\n创造力爆表🎨\n快来试试！\n\n图像9:16比例",
      "en": "Draw an image: Create a cover for a Xiaohongshu (RED) post.\n\nRequirements:\n– It must be visually compelling enough to attract user clicks.\n– Use bold, characterful fonts.\n– Vary font sizes to reflect the hierarchy of information; emphasize the structure of the copy.\n– The main title should be at least twice the size of regular text.\n– Leave white space between text sections.\n– Only use bright accent colors to highlight key words and draw attention.\n– The background should feature an eye-catching pattern (such as paper texture, notebook, or a WeChat chat window—choose one).\n– Add appropriate icons or illustrations to enhance visual layers, but avoid visual clutter.\n\nCopy text:\nBREAKING: ChatGPT just got even better!\n– Superior multitasking ✨\n– Stronger coding ability 💪\n– Creativity off the charts 🎨\nTry it now!\n\nImage aspect ratio: 9:16"
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1905507936526627078",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/22",
    "attributionUrl": "/creation/inspiration/attribution/case-22.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-22.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-21",
    "category": "illustration",
    "title": {
      "zh": "Q版角色表情包",
      "en": "Chibi Character Sticker Pack"
    },
    "prompt": {
      "zh": "请创作一套以 [参考图片中的角色] 为主角的Q版表情包，共9个，排列成3x3网格。\n设计要求：\n- 透明背景。\n- 1:1正方形构图。\n- 统一的Q版吉卜力卡通风格，色彩鲜艳。\n- 每个表情的动作、神态、内容各不相同，需要体现“骚、贱、萌、抓狂”等多样情绪，例如：翻白眼、捶地狂笑、灵魂出窍、原地石化、撒钱、干饭状态、社交恐惧发作等。可融入打工人和网络热梗元素。\n- 每个表情形象完整，无残缺。\n- 每个表情均带有统一的白色描边，呈现贴纸效果。\n- 画面中无多余、分离的元素。\n- 严格禁止出现任何文字，或确保文字内容准确无误（优先选择无文字）。",
      "en": "Please create a set of 9 Chibi stickers featuring [the character in the reference image], arranged in a 3x3 grid.\nDesign requirements:\n- Transparent background.\n- 1:1 square aspect ratio.\n- Consistent Chibi Ghibli cartoon style with vibrant colors.\n- Each sticker must have a unique action, expression, and theme, reflecting diverse emotions like \"sassy, mischievous, cute, frantic\" (e.g., rolling eyes, laughing hysterically on the floor, soul leaving body, petrified, throwing money, foodie mode, social anxiety attack). Incorporate elements related to office workers and internet memes.\n- Each character depiction must be complete, with no missing parts.\n- Each sticker must have a uniform white outline, giving it a sticker-like appearance.\n- No extraneous or detached elements in the image.\n- Strictly no text, or ensure any text is 100% accurate (no text preferred)."
    },
    "promptAuthor": "@leon_yuan2001",
    "promptAuthorUrl": "https://x.com/leon_yuan2001",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/leon_yuan2001/status/1923712069209293014",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/21",
    "attributionUrl": "/creation/inspiration/attribution/case-21.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-21.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-20",
    "category": "photography",
    "title": {
      "zh": "手办与真人同框",
      "en": "Action Figure and Real Person in the Same Frame"
    },
    "prompt": {
      "zh": "以手机随手拍摄的日常风格，桌面上摆放着一款 【成龙】动漫手办，动作夸张帅气，装备齐全。同时，真实世界的对应人物也出现在镜头中，与手办摆出相似的姿势，形成手办与真实人物同框的有趣对比效果。整体构图和谐自然，传递温暖且富有生活气息的视觉体验。",
      "en": "In a casual, everyday style as if shot on a mobile phone, an anime figure of [Jackie Chan] is placed on a desk, striking an exaggerated and cool pose, fully equipped. Simultaneously, the corresponding real-life person also appears in the frame, striking a similar pose to the figure, creating an interesting visual contrast with the figure and the real person in the same frame. The overall composition is harmonious and natural, delivering a warm and vibrant, true-to-life visual experience."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1904980568107819060",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/20",
    "attributionUrl": "/creation/inspiration/attribution/case-20.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-20.jpg",
    "width": 640,
    "height": 426
  },
  {
    "id": "gpt4o-19",
    "category": "architecture",
    "title": {
      "zh": "玩具盒中的国家立体模型",
      "en": "Country Diorama in a Toy Box"
    },
    "prompt": {
      "zh": "一张超写实的俯拍摄影作品，展示了一个米色纸板盒内的3D打印立体模型，盒盖由两只人手撑开。盒子内部展现了[国家名称]的微缩景观，包含标志性地标、地形、建筑、河流、植被以及大量微小精细的人物模型。该立体模型充满了鲜活且符合地理特征的元素，全部采用触感舒适、玩具般的风格，使用哑光3D打印纹理制作，并带有可见的打印层纹。在顶部，盒盖内侧用大号、色彩鲜艳的凸起塑料字母显示“[国家名称]”字样——每个字母颜色各异，均为亮色。光线温暖且具有电影感，突出了纹理和阴影，营造出一种真实感和魅力，仿佛观看者正在打开一个神奇的国家微缩版本。",
      "en": "An ultra-realistic top-down photograph of a 3D-printed diorama inside a beige cardboard box, with the lid being held open by two human hands. The interior of the box reveals a miniature landscape of [COUNTRY NAME], featuring iconic landmarks, terrain, buildings, rivers, vegetation, and crowds of tiny, detailed human figures. The diorama is filled with vibrant, geographically appropriate elements, all crafted in a tactile, toy-like style using matte 3D-printed textures with visible layer lines. At the top, the inside of the box lid displays the phrase “[COUNTRY NAME]” in large, colorful, raised plastic letters—each letter in a different bright color. The lighting is warm and cinematic, highlighting the textures and shadows to evoke a sense of realism and charm, as if the viewer is opening a magical miniature version of the nation"
    },
    "promptAuthor": "@TheRelianceAI",
    "promptAuthorUrl": "https://x.com/TheRelianceAI",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/TheRelianceAI/status/1925223613055017251",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/19",
    "attributionUrl": "/creation/inspiration/attribution/case-19.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-19.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-18",
    "category": "illustration",
    "title": {
      "zh": "皮克斯3D风格",
      "en": "Pixar 3D Style"
    },
    "prompt": {
      "zh": "以皮克斯 3D 风格重绘这张照片",
      "en": "Redraw this photo in Pixar 3D style"
    },
    "promptAuthor": "AnimeAI",
    "promptAuthorUrl": "https://animeai.online",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://animeai.online/#demo-gallery",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/18",
    "attributionUrl": "/creation/inspiration/attribution/case-18.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-18.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-17",
    "category": "product",
    "title": {
      "zh": "复古CRT电脑启动屏幕",
      "en": "Retro CRT Computer Boot Screen"
    },
    "prompt": {
      "zh": "复古CRT电脑启动屏幕，最终显示为[形状或标志]的ASCII艺术。",
      "en": "Retro CRT computer boot screen that resolves into ASCII-art of [shape or logo]"
    },
    "promptAuthor": "@Gdgtify",
    "promptAuthorUrl": "https://x.com/Gdgtify",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/Gdgtify/status/1925176250626159053",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/17",
    "attributionUrl": "/creation/inspiration/attribution/case-17.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-17.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-16",
    "category": "product",
    "title": {
      "zh": "二次元风格徽章",
      "en": "Anime-style Badge"
    },
    "prompt": {
      "zh": "基于附件中的人物，生成一个二次元风格的徽章的照片，要求：\n材质：流苏\n形状：圆形\n画面主体：一只手手持徽章",
      "en": "Based on the person in the attachment, generate a photo of an anime-style badge. Requirements:\nMaterial: Tassel\nShape: Circular\nMain subject: A hand holding the badge"
    },
    "promptAuthor": "@Alittlefatwhale",
    "promptAuthorUrl": "https://x.com/Alittlefatwhale",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/Alittlefatwhale/status/1922512847030124905",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/16",
    "attributionUrl": "/creation/inspiration/attribution/case-16.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-16.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-15",
    "category": "poster",
    "title": {
      "zh": "讽刺海报生成",
      "en": "Satirical Poster Generation"
    },
    "prompt": {
      "zh": "为我生成讽刺海报：GPT 4o 狂卷，都别干图像AI了 还是送外卖吧",
      "en": "Satirical Poster Text (English):\nGPT-4o is taking over.\nForget working in image AI\nmaybe it’s time to deliver takeout instead."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1905287637084274742",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/15",
    "attributionUrl": "/creation/inspiration/attribution/case-15.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-15.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-14",
    "category": "product",
    "title": {
      "zh": "《海贼王》主题手办制作",
      "en": "One Piece Themed Figure Creation"
    },
    "prompt": {
      "zh": "把照片中的人物变成《海贼王》（One Piece）动漫主题手办包装盒的风格，以等距视角（isometric）呈现。包装盒内展示的是基于照片人物的《海贼王》动漫画风设计的形象，旁边搭配有日常必备物品（手枪、手表、西装和皮鞋）同时，在包装盒旁边还应呈现该手办本体的实物效果，采用逼真的、具有真实感的渲染风格。",
      "en": "Transform the person in the photo into a One Piece-themed anime-style action figure, presented inside a collectible figure box designed in the visual style of the One Piece universe. The box is shown in an isometric view.\nInside the box, display the character reimagined in the One Piece anime art style, posed dynamically and accompanied by essential everyday items such as a pistol, a wristwatch, a suit, and leather shoes — all miniaturized and arranged like collectible accessories.\nNext to the box, include a realistic, fully rendered version of the actual figure itself, outside of the packaging. This figure should be rendered with high detail and realism, showcasing the material textures and craftsmanship, as if it were a professionally photographed product."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1909047547563213145",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/14",
    "attributionUrl": "/creation/inspiration/attribution/case-14.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-14.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-13",
    "category": "illustration",
    "title": {
      "zh": "3D Q版风格",
      "en": "Photo to 3D Q-version Style"
    },
    "prompt": {
      "zh": "将场景中的角色转化为3D Q版风格，同时保持原本的场景布置和服装造型不变。",
      "en": "Transform the characters in the scene into 3D chibi-style figures, while keeping the original scene layout and their clothing exactly the same."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1908194518345678865",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/13",
    "attributionUrl": "/creation/inspiration/attribution/case-13.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-13.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-12",
    "category": "product",
    "title": {
      "zh": "3D 情侣珠宝盒摆件",
      "en": "3D Couple Jewelry Box Figurine"
    },
    "prompt": {
      "zh": "根据照片上的内容打造一款细致精美、萌趣可爱的3D渲染收藏摆件，装置在柔和粉彩色调、温馨浪漫的展示盒中。展示盒为浅奶油色搭配柔和的金色装饰，形似精致的便携珠宝盒。打开盒盖，呈现出一幕温暖浪漫的场景：两位Q版角色正甜蜜相望。盒顶雕刻着“FOREVER TOGETHER”（永远在一起）的字样，周围点缀着小巧精致的星星与爱心图案。\n盒内站着照片上的女性，手中捧着一束小巧的白色花束。她的身旁是她的伴侣，照片上的男性。两人都拥有大而闪亮、充满表现力的眼睛，以及柔和、温暖的微笑，传递出浓浓的爱意和迷人的气质。\n他们身后有一扇圆形窗户，透过窗户能看到阳光明媚的中国古典小镇天际线和轻柔飘浮的云朵。盒内以温暖的柔和光线进行照明，背景中漂浮着花瓣点缀气氛。整个展示盒和角色的色调优雅和谐，营造出一个奢华而梦幻的迷你纪念品场景。\n尺寸：9:16",
      "en": "Create a finely crafted, adorably charming 3D-rendered collectible figure based on the subjects in the photo, displayed inside a pastel-toned, warm and romantic presentation box. The box is designed in a soft cream color with gentle gold accents, resembling an elegant portable jewelry case.\n\nWhen opened, the box reveals a heartwarming romantic scene: two chibi-style characters gazing sweetly at each other. The lid is engraved with the words “FOREVER TOGETHER,” surrounded by delicate star and heart motifs.\n\nInside the box stands the female from the photo, holding a small bouquet of white flowers. Beside her is her partner, the male from the photo. Both characters have large, expressive, sparkling eyes and soft, warm smiles that radiate affection and charm.\n\nBehind them is a round window, through which a sunny skyline of a traditional Chinese town can be seen, along with gently drifting clouds. The interior is softly lit with warm ambient lighting, and petals float in the background to enhance the atmosphere.\n\nThe overall color scheme of both the display box and the characters is elegant and harmonious, creating a luxurious and dreamlike miniature keepsake.\n\nAspect ratio: 9:16"
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1909332895115714835",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/12",
    "attributionUrl": "/creation/inspiration/attribution/case-12.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-12.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-11",
    "category": "poster",
    "title": {
      "zh": "PS2 游戏封面 (GTA x Shrek)",
      "en": "PS2 Game Cover (GTA x Shrek)"
    },
    "prompt": {
      "zh": "你能制作一个PS2游戏封面的图像吗？标题为《Grand Theft Auto: Far Far Away》。是一个设定在《怪物史瑞克》宇宙中的GTA风格游戏。",
      "en": "Can you create a PS2 video game case of \"Grand Theft Auto: Far Far Away\" a GTA based in the Shrek Universe."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1904978767090524372",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/11",
    "attributionUrl": "/creation/inspiration/attribution/case-11.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-11.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-10",
    "category": "illustration",
    "title": {
      "zh": "讽刺漫画生成",
      "en": "Satirical Cartoon Generation"
    },
    "prompt": {
      "zh": "一幅讽刺漫画风格的插画，采用复古美式漫画风格，背景是一个多层货架，货架上都是一样的红色棒球帽，帽子正面印有大字标语“MAKE AMERICA GREAT AGAIN”，帽侧贴着白色标签写着“MADE IN CHINA”，特写视角聚焦其中一顶红色棒球帽。画面下方有价格牌，原价“$50.00”被粗黑线X划掉，改为“$77.00”，色调为怀旧的土黄与暗红色调，阴影处理带有90年代复古印刷质感。整体构图风格夸张讽刺，具讽刺政治消费主义的意味。",
      "en": "An illustration in satirical comic style, rendered in a vintage American comic aesthetic. The background features a multi-tiered shelf stocked entirely with identical red baseball caps. The caps have a bold slogan on the front: “MAKE AMERICA GREAT AGAIN,” while a white side tag on each reads “MADE IN CHINA.” The composition uses a close-up perspective focusing on one specific red cap.\nAt the bottom of the image, a price label is shown: the original price “$50.00” is crossed out with a thick black X and replaced with “$77.00.” The overall color palette uses nostalgic ochre and deep red tones, with shading that mimics the textured print style of 1990s retro comics.\nThe composition is exaggerated and satirical, carrying a strong critique of political consumerism."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1910514811756065159",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/10",
    "attributionUrl": "/creation/inspiration/attribution/case-10.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-10.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-8",
    "category": "product",
    "title": {
      "zh": "乐高收藏品",
      "en": "Lego Collectible Figure"
    },
    "prompt": {
      "zh": "根据我上传的照片，生成一张纵向比例的照片，使用以下提示词：\n经典乐高人偶风格，一个微缩场景 —— 一只动物站在我身旁。这只动物的配色与我相匹配。\n请根据你对我的理解来创造这只动物（你可以选择任何你认为适合我的动物，不论是真实存在的，还是超现实的、幻想的，只要你觉得符合我的气质即可）。\n整个场景设定在一个透明玻璃立方体内，布景极简。\n微缩场景的底座是哑光黑色，配以银色装饰，风格简约且时尚。\n底座上有一块优雅雕刻的标签牌，字体为精致的衬线体，上面写着该动物的名称。\n底部设计中还巧妙融入了类似自然历史博物馆展示的生物学分类信息，以精细蚀刻的方式呈现。\n整体构图像是一件高端收藏艺术品：精心打造、策展般呈现、灯光细致。\n构图重在平衡。背景为渐变色，从深色到浅色过渡（颜色基于主色调进行选择）。",
      "en": "Generate a vertically-oriented image based on my uploaded photo, using the following prompt:\nClassic LEGO minifigure style in a miniature scene — an animal stands beside me. The color palette of the animal should match mine.\nPlease design the animal based on your understanding of me. You may choose any creature — real, surreal, or fantastical — that you feel best reflects my personality.\nThe entire scene is set within a transparent glass cube, with a minimalist interior design.\nThe base of the miniature is matte black with silver accents, following a clean and modern aesthetic.\nOn the base, there is an elegantly engraved nameplate in a refined serif font, displaying the name of the animal.\nThe lower part of the base subtly incorporates finely etched biological classification details, similar to a natural history museum display.\nThe overall composition should resemble a high-end collectible artwork: meticulously crafted, curated in style, and lit with refined lighting.\nBalance is key to the layout. The background should feature a smooth gradient transition from dark to light tones, selected to match the dominant color theme."
    },
    "promptAuthor": "@ZHO_ZHO_ZHO",
    "promptAuthorUrl": "https://x.com/ZHO_ZHO_ZHO",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/ZHO_ZHO_ZHO/status/1910644499354968091",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/8",
    "attributionUrl": "/creation/inspiration/attribution/case-8.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-8.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-6",
    "category": "illustration",
    "title": {
      "zh": "角色穿越传送门",
      "en": "Character Stepping Through Portal"
    },
    "prompt": {
      "zh": "照片中的角色的 3D Q 版形象穿过传送门，牵着观众的手，在将观众拉向前时动态地回头一看。传送门外的背景是观众的现实世界，一个典型的程序员的书房，有书桌，显示器和笔记本电脑，传送门内是角色所处的3D Q 版世界，细节可以参考照片，整体呈蓝色调，和现实世界形成鲜明对比。传送门散发着神秘的蓝色和紫色色调，是两个世界之间的完美椭圆形框架处在画面中间。从第三人称视角拍摄的摄像机角度，显示观看者的手被拉入角色世界。2：3 的宽高比。",
      "en": "A 3D chibi-style version of the person in the photo is stepping through a glowing portal, reaching out and holding the viewer’s hand. As the character pulls the viewer forward, they turn back with a dynamic glance, inviting the viewer into their world.\nBehind the portal is the viewer’s real-life environment: a typical programmer’s study with a desk, monitor, and laptop, rendered in realistic detail. Inside the portal lies the character’s 3D chibi world, inspired by the photo, with a cool blue color scheme that sharply contrasts with the real-world surroundings.\nThe portal itself is a perfectly elliptical frame glowing with mysterious blue and purple light, positioned at the center of the image as a gateway between the two worlds.\nThe scene is captured from a third-person perspective, clearly showing the viewer’s hand being pulled into the character’s world. Use a 2:3 aspect ratio."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1908910838636765204",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/6",
    "attributionUrl": "/creation/inspiration/attribution/case-6.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-6.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-5",
    "category": "illustration",
    "title": {
      "zh": "吉卜力风格",
      "en": "Ghibli Style"
    },
    "prompt": {
      "zh": "以吉卜力风格重绘这张照片",
      "en": "Redraw this photo in Ghibli style"
    },
    "promptAuthor": "AnimeAI",
    "promptAuthorUrl": "https://animeai.online",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://animeai.online/#demo-gallery",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/5",
    "attributionUrl": "/creation/inspiration/attribution/case-5.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-5.jpg",
    "width": 426,
    "height": 640
  },
  {
    "id": "gpt4o-4",
    "category": "illustration",
    "title": {
      "zh": "3D Q版中式婚礼图",
      "en": "3D Chibi Chinese Wedding Scene"
    },
    "prompt": {
      "zh": "将照片里的两个人转换成Q版 3D人物，中式古装婚礼，大红颜色，背景“囍”字剪纸风格图案。 服饰要求：写实，男士身着长袍马褂，主体为红色，上面以金色绣龙纹图案，彰显尊贵大气 ，胸前系着大红花，寓意喜庆吉祥。女士所穿是秀禾服，同样以红色为基调，饰有精美的金色花纹与凤凰刺绣，展现出典雅华丽之感 ，头上搭配花朵发饰，增添柔美温婉气质。二者皆为中式婚礼中经典着装，蕴含着对新人婚姻美满的祝福。 头饰要求： 男士：中式状元帽，主体红色，饰有金色纹样，帽顶有精致金饰，尽显传统儒雅庄重。 女士：凤冠造型，以红色花朵为中心，搭配金色立体装饰与垂坠流苏，华丽富贵，古典韵味十足。",
      "en": "Transform the two people in the photo into chibi-style 3D cartoon characters, dressed in traditional Chinese wedding attire. The overall theme is a festive red Chinese-style wedding. The background features a decorative “囍” (double happiness) paper-cut pattern in a classic folk style. \nClothing (realistic texture, traditional details):\nMale: Wearing a red changpao and magua (traditional robe and jacket) embroidered with golden dragon motifs, symbolizing nobility and grandeur. A large red flower is tied on his chest, representing celebration and good fortune.\nFemale: Dressed in a red xiuhe wedding gown adorned with exquisite golden floral and phoenix embroidery, showcasing elegance and luxury. She wears delicate floral hair ornaments to enhance her gentle and graceful appearance.\nHeadwear:\nMale: A traditional red zhuangyuan (scholar) hat with golden patterns and a refined golden ornament at the top, exuding classic scholarly dignity.\nFemale: A phoenix crown adorned with a central red flower, gold 3D decorative elements, and hanging tassels—luxurious and full of classical charm.\nThis image should reflect the joy and blessing of a traditional Chinese wedding, with realistic textures for costumes and accessories, combined with stylized 3D chibi characters."
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1909418699150237917",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/4",
    "attributionUrl": "/creation/inspiration/attribution/case-4.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-4.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-2",
    "category": "illustration",
    "title": {
      "zh": "3D Q版人物立体相框",
      "en": "3D Polaroid Breakout Effect"
    },
    "prompt": {
      "zh": "将场景中的角色转化为3D Q版风格，放在一张拍立得照片上，相纸被一只手拿着，照片中的角色正从拍立得照片中走出，呈现出突破二维相片边框、进入二维现实空间的视觉效果。",
      "en": "Convert the character in the scene into a 3D chibi-style figure, placed inside a Polaroid photo. The photo paper is being held by a human hand. The character is stepping out of the Polaroid frame, creating a visual effect of breaking through the two-dimensional photo border and entering the real-world 3D space."
    },
    "promptAuthor": "@dotey",
    "promptAuthorUrl": "https://x.com/dotey",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/dotey/status/1908238003169903060",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/2",
    "attributionUrl": "/creation/inspiration/attribution/case-2.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-2.jpg",
    "width": 640,
    "height": 640
  },
  {
    "id": "gpt4o-1",
    "category": "illustration",
    "title": {
      "zh": "Q版求婚场景",
      "en": "3D Chibi Proposal Scene"
    },
    "prompt": {
      "zh": "将照片里的两个人转换成Q版 3D人物，场景换成求婚，背景换成淡雅五彩花瓣做的拱门，背景换成浪漫颜色，地上散落着玫瑰花瓣。除了人物采用Q版 3D人物风格，其他环境采用真实写实风格。",
      "en": "Transform the two people in the photo into chibi-style 3D cartoon characters. Change the scene to a proposal setting, with a soft pastel-colored floral arch in the background. Use romantic tones for the overall background. Rose petals are scattered on the ground. While the characters are rendered in cute chibi 3D style, the environment—including the arch, lighting, and textures—should be realistic and photorealistic."
    },
    "promptAuthor": "@balconychy",
    "promptAuthorUrl": "https://x.com/balconychy",
    "imageAuthor": "@jamez-bondos",
    "imageAuthorUrl": "https://github.com/jamez-bondos",
    "sourceUrl": "https://x.com/balconychy/status/1909417750587486469",
    "repositoryUrl": "https://github.com/jamez-bondos/awesome-gpt4o-images/tree/3ed4dbb0f1ae6f7385c81e1fcea48d9eefd2e596/cases/1",
    "attributionUrl": "/creation/inspiration/attribution/case-1.yml",
    "license": "CC-BY-4.0",
    "creationTool": "Sora",
    "image": "/creation/inspiration/case-1.jpg",
    "width": 640,
    "height": 640
  }
]
