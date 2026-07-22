import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return `${JSON.stringify(obj, null, 2)}\n`
}

const newKeys = {
  "en": {
    "Cost multiplier for this upstream group.": "Cost multiplier for this upstream group.",
    "Current password": "Current password",
    "Fetch ratios from upstream and match this group to update the ratio field.": "Fetch ratios from upstream and match this group to update the ratio field.",
    "Fetch upstream ratios": "Fetch upstream ratios",
    "Failed to fetch upstream password": "Failed to fetch upstream password",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.",
    "Leave empty to keep existing password": "Leave empty to keep existing password",
    "Optional. Keep blank if this upstream uses token or session auth.": "Optional. Keep blank if this upstream uses token or session auth.",
    "Paste fetched upstream ratios JSON here": "Paste fetched upstream ratios JSON here",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "Raw upstream ratios JSON snapshot used for matching and sync.",
    "Reveal password": "Reveal password",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "Save upstream login information and pricing metadata for ratio sync and alerts.",
    "Shown in insufficient balance notifications.": "Shown in insufficient balance notifications.",
    "Upstream Group Ratios": "Upstream Group Ratios",
    "Upstream Information": "Upstream Information",
    "Upstream Login URL": "Upstream Login URL",
    "Upstream Password": "Upstream Password",
    "Upstream password unlocked": "Upstream password unlocked",
    "Upstream ratios updated": "Upstream ratios updated",
    "Used when logging in to the upstream site before fetching ratios.": "Used when logging in to the upstream site before fetching ratios.",
    "Verification required to reveal the saved upstream password.": "Verification required to reveal the saved upstream password.",
    "Paste Connection Info": "Paste Connection Info",
    "Verification scope is missing": "Verification scope is missing"
  },
  "zh": {
    "Cost multiplier for this upstream group.": "该上游分组的成本倍率。",
    "Current password": "当前密码",
    "Fetch ratios from upstream and match this group to update the ratio field.": "从上游拉取倍率，并按此分组匹配后更新倍率字段。",
    "Fetch upstream ratios": "获取上游倍率",
    "Failed to fetch upstream password": "获取上游密码失败",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "1 CNY 可购买多少等值 USD 的上游额度。留空或 0 表示 1。",
    "Leave empty to keep existing password": "留空则保留现有密码",
    "Optional. Keep blank if this upstream uses token or session auth.": "可选。如果该上游使用 token 或会话鉴权，可留空。",
    "Paste fetched upstream ratios JSON here": "将获取到的上游倍率 JSON 粘贴到这里",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "用于匹配和同步的上游倍率原始 JSON 快照。",
    "Reveal password": "显示密码",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "保存上游登录信息和价格元数据，用于倍率同步和告警。",
    "Shown in insufficient balance notifications.": "会显示在余额不足通知中。",
    "Upstream Group Ratios": "上游分组倍率",
    "Upstream Information": "上游信息",
    "Upstream Login URL": "上游登录地址",
    "Upstream Password": "上游密码",
    "Upstream password unlocked": "上游密码已解锁",
    "Upstream ratios updated": "上游倍率已更新",
    "Used when logging in to the upstream site before fetching ratios.": "在获取倍率前登录上游站点时使用。",
    "Verification required to reveal the saved upstream password.": "需要验证后才能显示已保存的上游密码。",
    "Paste Connection Info": "粘贴连接信息",
    "Verification scope is missing": "缺少验证范围"
  },
  "fr": {
    "Cost multiplier for this upstream group.": "Multiplicateur de coût pour ce groupe upstream.",
    "Current password": "Mot de passe actuel",
    "Fetch ratios from upstream and match this group to update the ratio field.": "Récupérez les ratios depuis l’upstream et faites correspondre ce groupe pour mettre à jour le champ du ratio.",
    "Fetch upstream ratios": "Récupérer les ratios upstream",
    "Failed to fetch upstream password": "Échec de la récupération du mot de passe upstream",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "Combien de crédits upstream équivalents en USD 1 CNY permet d’acheter. Vide ou 0 signifie 1.",
    "Leave empty to keep existing password": "Laissez vide pour conserver le mot de passe existant",
    "Optional. Keep blank if this upstream uses token or session auth.": "Facultatif. Laissez vide si cet upstream utilise une authentification par jeton ou session.",
    "Paste fetched upstream ratios JSON here": "Collez ici le JSON des ratios upstream récupérés",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "Instantané JSON brut des ratios upstream utilisé pour la correspondance et la synchronisation.",
    "Reveal password": "Afficher le mot de passe",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "Enregistrez les informations de connexion upstream et les métadonnées tarifaires pour la synchronisation des ratios et les alertes.",
    "Shown in insufficient balance notifications.": "Affiché dans les notifications de solde insuffisant.",
    "Upstream Group Ratios": "Ratios de groupe upstream",
    "Upstream Information": "Informations upstream",
    "Upstream Login URL": "URL de connexion upstream",
    "Upstream Password": "Mot de passe upstream",
    "Upstream password unlocked": "Mot de passe upstream déverrouillé",
    "Upstream ratios updated": "Ratios upstream mis à jour",
    "Used when logging in to the upstream site before fetching ratios.": "Utilisé lors de la connexion au site upstream avant de récupérer les ratios.",
    "Verification required to reveal the saved upstream password.": "Une vérification est requise pour afficher le mot de passe upstream enregistré.",
    "Paste Connection Info": "Coller les infos de connexion",
    "Verification scope is missing": "La portée de vérification est manquante"
  },
  "ja": {
    "Cost multiplier for this upstream group.": "このアップストリームグループのコスト倍率です。",
    "Current password": "現在のパスワード",
    "Fetch ratios from upstream and match this group to update the ratio field.": "アップストリームから倍率を取得し、このグループに一致した値で倍率欄を更新します。",
    "Fetch upstream ratios": "上流倍率を取得",
    "Failed to fetch upstream password": "アップストリームのパスワード取得に失敗しました",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "1 CNY で購入できる USD 相当のアップストリームクレジット量です。空欄または 0 は 1 を意味します。",
    "Leave empty to keep existing password": "既存のパスワードを保持するには空欄のままにしてください",
    "Optional. Keep blank if this upstream uses token or session auth.": "任意です。このアップストリームがトークンまたはセッション認証を使う場合は空欄のままにしてください。",
    "Paste fetched upstream ratios JSON here": "取得したアップストリーム倍率 JSON をここに貼り付けます",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "照合と同期に使うアップストリーム倍率の生 JSON スナップショットです。",
    "Reveal password": "パスワードを表示",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "倍率同期とアラートのために、アップストリームのログイン情報と価格メタデータを保存します。",
    "Shown in insufficient balance notifications.": "残高不足通知に表示されます。",
    "Upstream Group Ratios": "アップストリームグループ倍率",
    "Upstream Information": "アップストリーム情報",
    "Upstream Login URL": "アップストリームログイン URL",
    "Upstream Password": "アップストリームパスワード",
    "Upstream password unlocked": "アップストリームパスワードを表示しました",
    "Upstream ratios updated": "アップストリーム倍率を更新しました",
    "Used when logging in to the upstream site before fetching ratios.": "倍率取得前にアップストリームサイトへログインするときに使います。",
    "Verification required to reveal the saved upstream password.": "保存済みのアップストリームパスワードを表示するには認証が必要です。",
    "Paste Connection Info": "接続情報を貼り付け",
    "Verification scope is missing": "検証スコープがありません"
  },
  "ru": {
    "Cost multiplier for this upstream group.": "Коэффициент стоимости для этой upstream-группы.",
    "Current password": "Текущий пароль",
    "Fetch ratios from upstream and match this group to update the ratio field.": "Получите коэффициенты из upstream и сопоставьте эту группу, чтобы обновить поле коэффициента.",
    "Fetch upstream ratios": "Получить upstream-коэффициенты",
    "Failed to fetch upstream password": "Не удалось получить upstream-пароль",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "Сколько upstream-кредитов в эквиваленте USD можно купить за 1 CNY. Пусто или 0 означает 1.",
    "Leave empty to keep existing password": "Оставьте пустым, чтобы сохранить текущий пароль",
    "Optional. Keep blank if this upstream uses token or session auth.": "Необязательно. Оставьте пустым, если этот upstream использует аутентификацию по токену или сессии.",
    "Paste fetched upstream ratios JSON here": "Вставьте сюда JSON с полученными upstream-коэффициентами",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "Необработанный JSON-снимок upstream-коэффициентов для сопоставления и синхронизации.",
    "Reveal password": "Показать пароль",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "Сохраните данные входа upstream и ценовые метаданные для синхронизации коэффициентов и оповещений.",
    "Shown in insufficient balance notifications.": "Показывается в уведомлениях о недостаточном балансе.",
    "Upstream Group Ratios": "Коэффициенты upstream-групп",
    "Upstream Information": "Информация upstream",
    "Upstream Login URL": "URL входа upstream",
    "Upstream Password": "Пароль upstream",
    "Upstream password unlocked": "Upstream-пароль разблокирован",
    "Upstream ratios updated": "Upstream-коэффициенты обновлены",
    "Used when logging in to the upstream site before fetching ratios.": "Используется при входе на upstream-сайт перед получением коэффициентов.",
    "Verification required to reveal the saved upstream password.": "Чтобы показать сохранённый upstream-пароль, требуется подтверждение.",
    "Paste Connection Info": "Вставить данные подключения",
    "Verification scope is missing": "Область проверки не указана"
  },
  "vi": {
    "Cost multiplier for this upstream group.": "Hệ số chi phí cho nhóm upstream này.",
    "Current password": "Mật khẩu hiện tại",
    "Fetch ratios from upstream and match this group to update the ratio field.": "Lấy tỷ lệ từ upstream và khớp nhóm này để cập nhật trường tỷ lệ.",
    "Fetch upstream ratios": "Lấy tỷ lệ upstream",
    "Failed to fetch upstream password": "Không thể lấy mật khẩu upstream",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "1 CNY mua được bao nhiêu tín dụng upstream quy đổi theo USD. Để trống hoặc 0 nghĩa là 1.",
    "Leave empty to keep existing password": "Để trống để giữ nguyên mật khẩu hiện có",
    "Optional. Keep blank if this upstream uses token or session auth.": "Không bắt buộc. Để trống nếu upstream này dùng xác thực bằng token hoặc phiên.",
    "Paste fetched upstream ratios JSON here": "Dán JSON tỷ lệ upstream đã lấy vào đây",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "Ảnh chụp JSON tỷ lệ upstream thô dùng để đối chiếu và đồng bộ.",
    "Reveal password": "Hiện mật khẩu",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "Lưu thông tin đăng nhập upstream và siêu dữ liệu giá để đồng bộ tỷ lệ và cảnh báo.",
    "Shown in insufficient balance notifications.": "Hiển thị trong thông báo số dư không đủ.",
    "Upstream Group Ratios": "Tỷ lệ nhóm upstream",
    "Upstream Information": "Thông tin upstream",
    "Upstream Login URL": "URL đăng nhập upstream",
    "Upstream Password": "Mật khẩu upstream",
    "Upstream password unlocked": "Đã mở khóa mật khẩu upstream",
    "Upstream ratios updated": "Đã cập nhật tỷ lệ upstream",
    "Used when logging in to the upstream site before fetching ratios.": "Dùng khi đăng nhập vào trang upstream trước khi lấy tỷ lệ.",
    "Verification required to reveal the saved upstream password.": "Cần xác minh để hiển thị mật khẩu upstream đã lưu.",
    "Paste Connection Info": "Dán thông tin kết nối",
    "Verification scope is missing": "Thiếu phạm vi xác minh"
  },
  "zh-TW": {
    "Cost multiplier for this upstream group.": "此上游群組的成本倍率。",
    "Current password": "目前密碼",
    "Fetch ratios from upstream and match this group to update the ratio field.": "從上游取得倍率，並依此群組比對後更新倍率欄位。",
    "Fetch upstream ratios": "取得上游倍率",
    "Failed to fetch upstream password": "取得上游密碼失敗",
    "How many USD-equivalent upstream credits 1 CNY buys. Empty or 0 means 1.": "1 CNY 可購買多少等值 USD 的上游額度。留空或 0 表示 1。",
    "Leave empty to keep existing password": "留空則保留現有密碼",
    "Optional. Keep blank if this upstream uses token or session auth.": "選填。若此上游使用 token 或工作階段驗證，可留空。",
    "Paste Connection Info": "貼上連線資訊",
    "Paste fetched upstream ratios JSON here": "將取得的上游倍率 JSON 貼到這裡",
    "Raw upstream ratios JSON snapshot used for matching and sync.": "用於比對與同步的上游倍率原始 JSON 快照。",
    "Reveal password": "顯示密碼",
    "Save upstream login information and pricing metadata for ratio sync and alerts.": "儲存上游登入資訊與價格中繼資料，用於倍率同步與告警。",
    "Shown in insufficient balance notifications.": "會顯示在餘額不足通知中。",
    "Upstream Group Ratios": "上游群組倍率",
    "Upstream Information": "上游資訊",
    "Upstream Login URL": "上游登入網址",
    "Upstream Password": "上游密碼",
    "Upstream password unlocked": "上游密碼已解鎖",
    "Upstream ratios updated": "上游倍率已更新",
    "Used when logging in to the upstream site before fetching ratios.": "在取得倍率前登入上游站點時使用。",
    "Verification required to reveal the saved upstream password.": "需要驗證後才能顯示已儲存的上游密碼。",
    "Verification scope is missing": "缺少驗證範圍"
  }
}

async function main() {
  let totalAdded = 0

  for (const [locale, trans] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))

    let count = 0
    for (const [key, value] of Object.entries(trans)) {
      if (!Object.hasOwn(json.translation, key)) {
        json.translation[key] = value
        count++
      } else if (json.translation[key] !== value) {
        json.translation[key] = value
        count++
      }
    }

    if (count > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
      )
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }

    console.log(`${locale}: ${count} translations applied`)
    totalAdded += count
  }

  console.log(`\nTotal: ${totalAdded} translations applied`)
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
