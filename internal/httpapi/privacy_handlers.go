package httpapi

import "net/http"

// privacyPolicy serves the Privacy Policy. The app links users straight here
// from the profile screen, so the document is published in Russian, Kyrgyz and
// English; all three versions state the same policy.
func (s *Server) privacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; script-src 'none'")
	setSiteSecurityHeaders(w)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(privacyPolicyHTML))
}

const privacyPolicyHTML = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>KOOM — Политика конфиденциальности / Купуялык саясаты / Privacy Policy</title>
  <style>
    :root{color-scheme:light;--bg:#f7f9ff;--card:#ffffff;--text:#10254f;--muted:#5f6e8a;--line:#dce6f5;--brand:#0b5ed7;--r-sm:12px;--r-md:16px;--r-lg:24px}
    *{box-sizing:border-box}
    html{scroll-behavior:smooth}
    body{margin:0;background:linear-gradient(180deg,#edf5ff 0,#f7f9ff 280px);color:var(--text);font-family:Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;font-weight:400;line-height:1.7}
    a:focus-visible{outline:3px solid var(--brand);outline-offset:3px;border-radius:var(--r-sm)}
    .policy-top{height:72px;display:flex;align-items:center;justify-content:space-between;padding:0 max(22px,calc((100% - 980px)/2));border-bottom:1px solid #dce6f5;background:#f7f9ffeb}
    .policy-brand{display:flex;align-items:center;gap:10px;color:var(--text);font-size:20px;font-weight:700;letter-spacing:-.02em;text-decoration:none}
    .policy-brand img{width:36px;height:36px;border-radius:var(--r-sm)}
    .policy-home{color:#506383;font-size:13px;font-weight:600;text-decoration:none}.policy-home:hover{color:var(--brand)}
    main{max-width:980px;margin:0 auto;padding:38px 18px 64px}
    .card{background:var(--card);border:1px solid var(--line);border-radius:var(--r-lg);padding:clamp(24px,5vw,46px);box-shadow:0 18px 55px rgba(17,45,86,.08);margin-bottom:28px;scroll-margin-top:90px}
    h1{font-size:clamp(28px,5vw,38px);line-height:1.15;letter-spacing:-.022em;font-weight:700;margin:0 0 8px}
    h2{font-size:20px;letter-spacing:-.01em;font-weight:600;margin:30px 0 10px;border-top:1px solid var(--line);padding-top:22px}
    p,li{font-size:15px} ul{padding-left:22px} li{padding-left:3px;margin:6px 0}.muted{color:var(--muted)} a{color:var(--brand)}
    .lang{display:flex;flex-wrap:wrap;gap:14px;margin:0 0 22px;font-weight:600;font-size:14px}
    footer{max-width:980px;margin:0 auto;padding:0 18px 40px;color:var(--muted);font-size:13px}
    @media(max-width:600px){.policy-top{height:64px;padding:0 18px}main{padding:22px 14px 40px}.card{border-radius:var(--r-md)}p,li{font-size:14px}}
    @media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}}
  </style>
</head>
<body>
<header class="policy-top"><a class="policy-brand" href="/"><img src="/assets/koom-icon.png" alt="">KOOM</a><a class="policy-home" href="/">KOOM ↗</a></header>
<main>
  <nav class="lang" aria-label="Язык / Тил / Language"><a href="#ru">Русский</a><a href="#ky" lang="ky">Кыргызча</a><a href="#en" lang="en">English</a></nav>

  <article class="card" id="ru" lang="ru">
    <h1>Политика конфиденциальности KOOM</h1>
    <p class="muted">Последнее обновление: 17 ноября 2026 г.</p>
    <p>KOOM — приложение для общения сообществ: группы, публикации, комментарии, голосования, обращения, обмен медиафайлами, уведомления и связанные с ними функции сообщества. Эта Политика конфиденциальности объясняет, какую информацию мы собираем, как мы её используем и какие возможности выбора есть у пользователей.</p>

    <h2>1. Какую информацию мы собираем</h2>
    <p>В зависимости от того, как вы используете KOOM, мы можем собирать следующие категории информации:</p>
    <ul>
      <li><strong>Данные аккаунта и контактные данные:</strong> номер телефона, используемый для входа и подтверждения, идентификатор пользователя, отображаемое имя, данные профиля, участие в группах и роль в приложении.</li>
      <li><strong>Контент, создаваемый пользователями:</strong> группы, публикации, обращения, комментарии, голоса, сообщения, файлы, изображения, видео и другие медиафайлы, которые вы создаёте, загружаете или публикуете в приложении.</li>
      <li><strong>Действия в приложении:</strong> действия, необходимые для работы функций приложения, — создание публикаций, присоединение к группам, отправка обращений, голосование, комментирование, использование функций модерации и администрирования.</li>
      <li><strong>Данные об устройстве и технические данные:</strong> версия приложения, тип устройства, операционная система, IP-адрес, журналы запросов, журналы ошибок, события безопасности, диагностика и данные о производительности.</li>
      <li><strong>Данные уведомлений:</strong> токены push-уведомлений и настройки уведомлений, если уведомления включены на вашем устройстве.</li>
    </ul>

    <h2>2. Как мы используем информацию</h2>
    <p>Мы используем информацию, чтобы:</p>
    <ul>
      <li>создавать и подтверждать учётные записи пользователей и управлять ими;</li>
      <li>обеспечивать работу групп, публикаций, комментариев, голосований, обращений, сообщений, обмена медиафайлами и связанных функций приложения;</li>
      <li>отправлять уведомления, служебные сообщения и важные обновления, касающиеся аккаунта или безопасности;</li>
      <li>предотвращать злоупотребления, спам, мошенничество, несанкционированный доступ и инциденты безопасности;</li>
      <li>модерировать контент и применять правила приложения;</li>
      <li>выявлять ошибки, поддерживать надёжность, улучшать производительность и разрабатывать новые функции;</li>
      <li>соблюдать юридические, операционные требования и требования безопасности.</li>
    </ul>

    <h2>3. Передача информации</h2>
    <p>Мы не продаём персональные данные. Мы можем передавать информацию проверенным поставщикам услуг, которые помогают нам обеспечивать работу приложения, или поручать им её обработку: это хостинг, базы данных, хранение файлов, безопасность, доставка уведомлений, аналитика, диагностика и поддержка. Мы также можем раскрывать информацию, когда это требуется по закону, для защиты пользователей, для расследования злоупотреблений или для защиты безопасности и целостности KOOM.</p>

    <h2>4. Публичный и групповой контент</h2>
    <p>Контент, отправленный в группы, обращения, комментарии, голосования, сообщения или аналогичные функции, может быть виден другим пользователям — в зависимости от функции, настроек группы, ролей и прав доступа. Не публикуйте информацию, которую вы не хотите показывать другим пользователям, имеющим к ней доступ.</p>

    <h2>5. Срок хранения данных</h2>
    <p>Мы храним информацию столько, сколько необходимо для работы KOOM, обеспечения безопасности, разрешения споров, применения правил, улучшения сервиса и выполнения юридических или операционных требований. Отдельные технические журналы и резервные копии могут храниться ограниченное время в целях безопасности, устранения неполадок и восстановления. Когда информация больше не нужна, мы удаляем её или обезличиваем, если это разумно возможно.</p>

    <h2>6. Безопасность</h2>
    <p>Мы применяем разумные технические и организационные меры для защиты информации от несанкционированного доступа, потери, неправомерного использования, изменения или раскрытия. Ни один онлайн-сервис не может гарантировать полную безопасность, но мы работаем над защитой учётных записей пользователей, данных приложения и инфраструктуры сервиса.</p>

    <h2>7. Выбор пользователя и удаление аккаунта</h2>
    <p>Вы можете удалить свою учётную запись в настройках профиля в приложении KOOM или начать удаление на странице <a href="/account-deletion">koom.com.kg/account-deletion</a>. При удалении удаляются ваш профиль, номер телефона, активные сессии, push-токены, голоса, сообщения в групповых чатах и другие записи, связанные с учётной записью. Обращения и комментарии остаются, но сведения об авторе удаляются; их текст и прикреплённые медиафайлы остаются частью публичной записи. Группы, которыми вы владеете, передаются существующему администратору или участнику группы. Если вы единственный участник группы, пригласите другого участника до удаления учётной записи.</p>
    <p>Удаление учётной записи из действующего сервиса может не сразу удалить информацию из защищённых резервных копий. Резервные копии удаляются согласно установленному для них сроку хранения. По вопросам доступа к данным или их исправления, а также если у вас нет доступа к номеру телефона, указанному в аккаунте, напишите нам: <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>

    <h2>8. Дети</h2>
    <p>KOOM не предназначен для детей младше минимального возраста, установленного применимым законодательством. Если нам станет известно, что персональные данные ребёнка были собраны без надлежащего согласия там, где согласие требуется, мы примем разумные меры для их удаления.</p>

    <h2>9. Международная обработка данных</h2>
    <p>Информация может обрабатываться и храниться на серверах или в сервисах, расположенных за пределами вашей страны. Мы принимаем разумные меры для защиты информации в соответствии с этой Политикой конфиденциальности, где бы она ни обрабатывалась.</p>

    <h2>10. Изменения этой политики</h2>
    <p>Мы можем время от времени обновлять эту Политику конфиденциальности. Когда мы вносим изменения, мы обновляем дату в начале этой страницы. Продолжение использования KOOM после обновления означает, что применяется обновлённая политика.</p>

    <h2>11. Контакты</h2>
    <p>По вопросам конфиденциальности и обращениям пишите нам: <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>
  </article>

  <article class="card" id="ky" lang="ky">
    <h1>KOOM купуялык саясаты</h1>
    <p class="muted">Акыркы жаңыртуу: 2026-жылдын 17-ноябры.</p>
    <p>KOOM — жамааттардын байланышы үчүн тиркеме: топтор, жарыялар, комментарийлер, добуш берүү, кайрылуулар, медиа файлдарды бөлүшүү, билдирүүлөр жана ушуларга байланышкан жамаат функциялары. Бул Купуялык саясаты биз кандай маалымат чогултканыбызды, аны кантип колдонгонубузду жана колдонуучулардын кандай тандоосу бар экенин түшүндүрөт.</p>

    <h2>1. Биз кандай маалымат чогултабыз</h2>
    <p>KOOMду кантип колдонгонуңузга жараша биз төмөнкү категориядагы маалыматтарды чогултушубуз мүмкүн:</p>
    <ul>
      <li><strong>Аккаунттун жана байланыш маалыматтары:</strong> кирүү жана ырастоо үчүн колдонулган телефон номери, колдонуучунун идентификатору, көрсөтүлүүчү аты, профилдин маалыматтары, топтордогу мүчөлүгү жана тиркемедеги ролу.</li>
      <li><strong>Колдонуучулар түзгөн мазмун:</strong> топтор, жарыялар, кайрылуулар, комментарийлер, добуштар, билдирүүлөр, файлдар, сүрөттөр, видеолор жана тиркемеде түзгөн, жүктөгөн же бөлүшкөн башка медиа файлдар.</li>
      <li><strong>Тиркемедеги аракеттер:</strong> тиркеменин функциялары иштеши үчүн керектүү аракеттер — жарыя түзүү, топторго кошулуу, кайрылуу жөнөтүү, добуш берүү, комментарий жазуу, модерация жана администрациялоо функцияларын колдонуу.</li>
      <li><strong>Түзмөк жана техникалык маалыматтар:</strong> тиркеменин версиясы, түзмөктүн түрү, операциялык система, IP-дарек, сурамдардын журналдары, каталардын журналдары, коопсуздук окуялары, диагностика жана иштөө көрсөткүчтөрү.</li>
      <li><strong>Билдирүү маалыматтары:</strong> push-билдирүүлөрдүн токендери жана билдирүү жөндөөлөрү, эгерде түзмөгүңүздө билдирүүлөр иштетилген болсо.</li>
    </ul>

    <h2>2. Маалыматты кантип колдонобуз</h2>
    <p>Биз маалыматты төмөнкүлөр үчүн колдонобуз:</p>
    <ul>
      <li>колдонуучулардын аккаунттарын түзүү, ырастоо жана башкаруу;</li>
      <li>топторду, жарыяларды, комментарийлерди, добуш берүүнү, кайрылууларды, билдирүүлөрдү, медиа файлдарды бөлүшүүнү жана ушуларга байланышкан функцияларды камсыздоо;</li>
      <li>билдирүүлөрдү, кызматтык билдирүүлөрдү жана аккаунт же коопсуздук тууралуу маанилүү жаңыртууларды жөнөтүү;</li>
      <li>кыянаттыкты, спамды, алдамчылыкты, уруксатсыз кирүүнү жана коопсуздук инциденттерин алдын алуу;</li>
      <li>мазмунду модерациялоо жана тиркеменин эрежелерин колдонуу;</li>
      <li>каталарды аныктоо, ишенимдүүлүктү сактоо, иштөө ылдамдыгын жакшыртуу жана жаңы функцияларды иштеп чыгуу;</li>
      <li>укуктук, операциялык жана коопсуздук талаптарын аткаруу.</li>
    </ul>

    <h2>3. Маалыматты берүү</h2>
    <p>Биз жеке маалыматтарды сатпайбыз. Тиркеменин ишин камсыздоого жардам берген ишенимдүү кызмат көрсөтүүчүлөр менен маалыматты бөлүшүшүбүз же аларга иштетүүгө беришибиз мүмкүн: хостинг, маалымат базалары, файл сактоо, коопсуздук, билдирүүлөрдү жеткирүү, аналитика, диагностика жана колдоо. Ошондой эле мыйзам талап кылганда, колдонуучуларды коргоо үчүн, кыянаттыкты тергөө үчүн же KOOMдун коопсуздугун жана бүтүндүгүн коргоо үчүн маалыматты ачыкка чыгарышыбыз мүмкүн.</p>

    <h2>4. Жалпыга ачык жана топтогу мазмун</h2>
    <p>Топторго, кайрылууларга, комментарийлерге, добуш берүүгө, билдирүүлөргө же ушуга окшош функцияларга жөнөтүлгөн мазмун — функцияга, топтун жөндөөлөрүнө, ролдорго жана уруксаттарга жараша — башка колдонуучуларга көрүнүшү мүмкүн. Уруксаты бар башка колдонуучуларга көрсөткүңүз келбеген маалыматты жарыялабаңыз.</p>

    <h2>5. Маалыматтарды сактоо мөөнөтү</h2>
    <p>Биз маалыматты KOOMдун иштеши, коопсуздукту сактоо, талаш-тартыштарды чечүү, эрежелерди колдонуу, кызматты жакшыртуу жана укуктук же операциялык талаптарды аткаруу үчүн канча керек болсо, ошончо убакыт сактайбыз. Айрым техникалык журналдар жана камдык көчүрмөлөр коопсуздук, кыйынчылыктарды четтетүү жана калыбына келтирүү максатында чектелген мөөнөткө сакталышы мүмкүн. Маалымат мындан ары керек болбой калганда, биз аны өчүрөбүз же акылга сыярлык мүмкүн болсо, жеке башын аныктоого болбой турган түргө келтиребиз.</p>

    <h2>6. Коопсуздук</h2>
    <p>Биз маалыматты уруксатсыз кирүүдөн, жоготуудан, туура эмес колдонуудан, өзгөртүүдөн же ачыкка чыгаруудан коргоо үчүн акылга сыярлык техникалык жана уюштуруу чараларын колдонобуз. Бир дагы онлайн кызмат толук коопсуздукка кепилдик бере албайт, бирок биз колдонуучулардын аккаунттарын, тиркеменин маалыматтарын жана кызматтын инфраструктурасын коргоо үчүн иштейбиз.</p>

    <h2>7. Колдонуучунун тандоосу жана аккаунтту өчүрүү</h2>
    <p>Аккаунтуңузду KOOM тиркемесинин профиль жөндөөлөрүнөн өчүрө аласыз же өчүрүүнү <a href="/account-deletion">koom.com.kg/account-deletion</a> барагынан баштасаңыз болот. Өчүрүү профилиңизди, телефон номериңизди, активдүү сессияларды, push-токендерди, добуштарды, топтук чаттардагы билдирүүлөрдү жана аккаунтка байланышкан башка жазууларды алып салат. Кайрылуулар жана комментарийлер калат, бирок автордун маалыматы алынып салынат; алардын тексти жана тиркелген медиа файлдары жалпыга ачык жазуунун бир бөлүгү болуп кала берет. Сиз ээси болгон топтор топтун бар администраторуна же мүчөсүнө өткөрүлүп берилет. Эгерде топтун жалгыз мүчөсү сиз болсоңуз, аккаунтуңузду өчүрүүдөн мурун башка мүчөнү чакырыңыз.</p>
    <p>Аккаунтту иштеп турган кызматтан өчүрүү корголгон камдык көчүрмөлөрдөн маалыматты дароо алып салбашы мүмкүн. Камдык көчүрмөлөр аларга белгиленген сактоо мөөнөтүнө ылайык өчүрүлөт. Маалыматка жетүү же аны оңдоо боюнча өтүнүчтөр үчүн, ошондой эле аккаунтуңузда көрсөтүлгөн телефон номерине кире албай калсаңыз, бизге жазыңыз: <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>

    <h2>8. Балдар</h2>
    <p>KOOM колдонуудагы мыйзам талап кылган минималдуу жаштан кичүү балдар үчүн арналган эмес. Макулдук талап кылынган учурда баланын жеке маалыматы тиешелүү макулдуксуз чогултулганын билсек, аны өчүрүү үчүн акылга сыярлык чараларды көрөбүз.</p>

    <h2>9. Эл аралык иштетүү</h2>
    <p>Маалымат сиздин мамлекетиңизден тышкары жайгашкан серверлерде же кызматтарда иштетилиши жана сакталышы мүмкүн. Маалымат кайда иштетилсе да, биз аны ушул Купуялык саясатына ылайык коргоо үчүн акылга сыярлык чараларды көрөбүз.</p>

    <h2>10. Бул саясаттагы өзгөртүүлөр</h2>
    <p>Биз бул Купуялык саясатын убакыт өткөн сайын жаңыртышыбыз мүмкүн. Өзгөртүү киргизгенде, бул барактын башындагы датаны жаңыртабыз. Жаңыртуудан кийин KOOMду колдонууну улантуу жаңыртылган саясат колдонулат дегенди билдирет.</p>

    <h2>11. Байланыш</h2>
    <p>Купуялык боюнча суроолор жана өтүнүчтөр үчүн бизге жазыңыз: <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>
  </article>

  <article class="card" id="en" lang="en">
    <h1>Koom Privacy Policy</h1>
    <p class="muted">Last updated: November 17, 2026</p>
    <p>Koom is a community communication application for groups, posts, comments, voting, public requests, media sharing, notifications, and related community features. This Privacy Policy explains what information we collect, how we use it, and what choices users have.</p>

    <h2>1. Information we collect</h2>
    <p>Depending on how you use Koom, we may collect the following categories of information:</p>
    <ul>
      <li><strong>Account and contact information:</strong> phone number used for sign-in and verification, user ID, display name, profile details, group membership, and app role.</li>
      <li><strong>User-generated content:</strong> groups, posts, public requests, comments, votes, messages, files, images, videos, and other media that you choose to create, upload, or share in the app.</li>
      <li><strong>App activity:</strong> actions needed to provide app features, such as creating posts, joining groups, submitting public requests, voting, commenting, and using moderation or administration features.</li>
      <li><strong>Device and technical information:</strong> app version, device type, operating system, IP address, request logs, error logs, security events, diagnostics, and performance information.</li>
      <li><strong>Notification data:</strong> push notification tokens and notification preferences if notifications are enabled on your device.</li>
    </ul>

    <h2>2. How we use information</h2>
    <p>We use information to:</p>
    <ul>
      <li>create, verify, and manage user accounts;</li>
      <li>provide groups, posts, comments, voting, public requests, messages, media sharing, and related app features;</li>
      <li>send notifications, service messages, and important account or security updates;</li>
      <li>prevent abuse, spam, fraud, unauthorized access, and security incidents;</li>
      <li>moderate content and enforce app rules;</li>
      <li>diagnose errors, maintain reliability, improve performance, and develop new features;</li>
      <li>comply with legal, security, and operational requirements.</li>
    </ul>

    <h2>3. Sharing of information</h2>
    <p>We do not sell personal information. We may share or process information with trusted service providers that help us operate the app, including hosting, databases, storage, security, notification delivery, analytics, diagnostics, and support. We may also disclose information when required by law, to protect users, to investigate abuse, or to protect the security and integrity of Koom.</p>

    <h2>4. Public and group content</h2>
    <p>Content submitted to groups, public requests, comments, votes, messages, or similar features may be visible to other users depending on the feature, group settings, roles, and permissions. Do not submit information that you do not want other permitted users to see.</p>

    <h2>5. Data retention</h2>
    <p>We keep information for as long as needed to provide Koom, maintain security, resolve disputes, enforce rules, improve the service, and meet legal or operational requirements. Some technical logs and backups may be retained for a limited period for security, troubleshooting, and recovery purposes. When information is no longer needed, we delete it or de-identify it where reasonably possible.</p>

    <h2>6. Security</h2>
    <p>We use reasonable technical and organizational measures to protect information against unauthorized access, loss, misuse, alteration, or disclosure. No online service can be guaranteed to be completely secure, but we work to protect user accounts, app data, and service infrastructure.</p>

    <h2>7. User choices and account deletion</h2>
    <p>You can delete your account from Profile settings in the Koom app or start deletion at <a href="/account-deletion">koom.com.kg/account-deletion</a>. Deletion removes your profile, phone number, active sessions, push tokens, votes, group chat messages, and other account-linked records. Public requests and comments remain with the author identity removed; their text and attached media remain part of the public record. Groups you own are transferred to an existing group admin or member. If you are the only member of a group, invite another member before deleting your account.</p>
    <p>Account deletion from the live service may not immediately remove information from protected backups. Backup copies are removed according to their configured retention schedule. For access or correction requests, or if you cannot access the phone number on your account, contact us at <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>

    <h2>8. Children</h2>
    <p>Koom is not intended for children under the minimum age required by applicable law. If we learn that personal information from a child was collected without appropriate consent where consent is required, we will take reasonable steps to delete it.</p>

    <h2>9. International processing</h2>
    <p>Information may be processed and stored on servers or services located outside your country. We take reasonable steps to protect information according to this Privacy Policy wherever it is processed.</p>

    <h2>10. Changes to this policy</h2>
    <p>We may update this Privacy Policy from time to time. When we make changes, we will update the date at the top of this page. Continued use of Koom after an update means the updated policy applies.</p>

    <h2>11. Contact</h2>
    <p>For privacy questions or requests, contact us at <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>
  </article>
</main>
<footer><a href="/terms">Условия использования / Колдонуу шарттары</a> · <a href="/account-deletion">Удаление аккаунта / Аккаунтту өчүрүү</a> · <a href="/child-safety">Child safety</a></footer>
</body>
</html>`
