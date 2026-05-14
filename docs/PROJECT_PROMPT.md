# MobileChat Server Product Prompt

Original request saved from the user:

> Нужно сделать приложение на языке Dart, чтобы был кроссплатформенный и для Android, и для iOS. И смотри, это приложение должно быть как Telegram, то есть все работы как в Telegram, но должно быть вот так вот. Личные чаты не должно быть, ну то есть будут только группы. Есть два вида групп. Первый только через код или через приглашение можно зайти у админа, а второй для всех. Ну то есть можешь поискать в поиске и туда войти. Давай, начинай сделать. Вот у тебя есть две репозитория, один для серверной части, другой для клиентской части. Сделай все красиво, как в Telegram. Все работа как в Telegram, но там не будет личных чатов, только будут группы. И человека можно добавить в группу по ID. То есть у каждого человека будет ID, всем будет виден. Можешь приглашать в группу через ID. Давай начинай, потом постепенно будем исправлять. Вот этот промпт обязательно где-нибудь сохрани. Не забывай об этом. Обязательно сохрани где-нибудь.

## Server responsibilities

- Manage users with visible public user IDs.
- Manage group-only chats.
- Support public groups searchable by everyone.
- Support private groups joinable by invite code or admin invitation.
- Allow group admins to add users by visible user ID.
- Provide chat message API.

## Initial MVP API

- `POST /api/auth/login`
- `GET /api/groups?user_id=...`
- `GET /api/groups/search?q=...`
- `POST /api/groups`
- `POST /api/groups/{group_id}/join`
- `POST /api/groups/join-by-code`
- `POST /api/groups/{group_id}/invite-user`
- `GET /api/groups/{group_id}/messages`
- `POST /api/groups/{group_id}/messages`
