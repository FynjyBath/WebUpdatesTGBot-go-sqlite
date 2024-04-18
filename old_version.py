import asyncio
import logging
from aiogram import *
from aiogram.filters.command import Command
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup
from aiogram.types import (
    KeyboardButton,
    Message,
    ReplyKeyboardMarkup,
    ReplyKeyboardRemove,
)
import sqlite3
import sys
from aiogram.enums import ParseMode
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from bs4 import BeautifulSoup as bs
import requests
from aiogram.utils.markdown import link
from aiogram.types import BotCommand

scheduler = AsyncIOScheduler()
router = Router()
con = sqlite3.connect('database.db')
cur = con.cursor()

@router.message(Command("start"))
async def start_handler(msg: types.Message):
    cur = con.cursor()
    await msg.answer("Привет!\nСписок доступных команд:\n✏️ /add - добавление URL\n🗑️ /del - удаление URL\n🚀 /nemalo - получение ссылки на канал Немало")
    user = cur.execute("SELECT * FROM users WHERE user_id=?", (msg.from_user.id,)).fetchall()
    if len(user) == 0:
        cur.execute(f"""INSERT INTO users(user_id, sites)
                            VALUES (?, "")""", (msg.from_user.id,))
        con.commit()


@router.message(Command("nemalo"))
async def start_handler(msg: types.Message):
    await msg.answer("@nemalo_i_tochka")


async def add_url(user_id, url):
    cur = con.cursor()
    sites = cur.execute("SELECT * FROM users WHERE user_id=?", (user_id,)).fetchone()[1].strip().split(",")
    if get_data(url) == "":
        return 0
    if len(sites) > 10:
        return 1
    sites.append(url)
    cur.execute(f'''UPDATE users
                    SET sites = ?
                    WHERE user_id = ?''', (",".join(sites).strip(', '), user_id))
    con.commit()
    return 2

async def del_url(user_id, num):
    cur = con.cursor()
    if not num.isdigit():
        return False
    num = int(num) - 1
    sites = cur.execute("SELECT * FROM users WHERE user_id=?", (user_id,)).fetchone()[1].strip().split(",")
    if num < 0 or num >= len(sites):
        return False
    sites.pop(num)
    cur.execute(f'''UPDATE users
                    SET sites = ?
                    WHERE user_id = ?''', (",".join(sites).strip(', '), user_id))
    con.commit()
    return True



def get_data(url):
    try:
        url = requests.get(url)
        s = bs(url.text, "html.parser").get_text('\n')
        s = s.replace('"', "'").replace('`', "'").replace('.', ' ').replace('*', ' ').replace('-', ' ').replace(': ', ':').replace(' :', ':').replace(':\n', ':').replace('\n:', ':')
        s_clear = ''
        i = 0
        while i < len(s):
            if s[i:i + 2].isdigit() and s[i + 3:i + 5].isdigit() and s[i + 6:i + 8].isdigit() and s[i + 2] == s[i + 5] == ':':
                i += 8
                continue
            if s[i:i + 3].isdigit() and s[i + 4:i + 6].isdigit() and s[i + 7:i + 9].isdigit() and s[i + 3] == s[i + 6] == ':':
                i += 9
                continue
            if s[i:i + 4].isdigit() and s[i + 5:i + 7].isdigit() and s[i + 8:i + 10].isdigit() and s[i + 4] == s[i + 7] == ':':
                i += 10
                continue
            if s[i:i + 2].isdigit() and s[i + 3:i + 5].isdigit() and s[i + 2] == ':':
                i += 5
                continue
            if s[i:i + 2].isdigit() and s[i + 3:i + 5].isdigit() and s[i + 6:i + 10].isdigit() and s[i + 2] == s[i + 5] == '.':
                i += 10
                continue
            if s[i:i + 2].isdigit() and s[i + 3:i + 5].isdigit() and s[i + 6:i + 8].isdigit() and s[i + 2] == s[i + 5] == '.':
                i += 8
                continue
            if s_clear == '' or (s_clear[-1] not in ' \n\t' or s[i] not in ' \n\t'):
                s_clear += s[i]
            i += 1
        return s_clear.strip()
    except Exception:
       return ""



def get_update(data1, data2):
    pref = 0
    while pref < min(len(data1), len(data2)) and data1[pref] == data2[pref]:
        pref += 1
    suf = 0
    while suf < min(len(data1), len(data2)) and data1[-suf - 1] == data2[-suf - 1]:
        suf += 1
    return data1[pref:len(data1) - suf], data2[pref:len(data2) - suf]

async def check_updates():
    already_checked = {}
    for (user_id, sites) in cur.execute("SELECT * FROM users").fetchall():
        sites = sites.split(',')
        for site in sites:
            if site in already_checked.keys():
                prev, now = already_checked[site]
                await bot.send_message(user_id, f"""ИЗМЕНЕНИЕ НА САЙТЕ: {link('URL', site)}
БЫЛО:
```
{prev[:min(len(prev), 100)] + ',,,' * (len(prev) > 100)}```
СТАЛО:
```
{now[:min(len(now), 100)] + ',,,' * (len(now) > 100)}```""", parse_mode = 'MarkdownV2', disable_web_page_preview=True)               
                continue

            data_now = get_data(site)

            data_prev = cur.execute("SELECT * FROM sites WHERE site = ?", (site,)).fetchall()
            if len(data_prev) == 0:
                data_prev = ''
                cur.execute(f'INSERT INTO sites(site, data) VALUES (?, "")', (site,))
                con.commit()
                flag = False
            else:
                data_prev = data_prev[0][1]
                flag = True

            if data_now == data_prev:
                continue

            prev, now = get_update(data_prev, data_now)
            already_checked[site] = prev, now

            if flag:
                await bot.send_message(user_id, f"""ИЗМЕНЕНИЕ НА: {link('URL', site)} 🔗
БЫЛО:
```
{prev[:min(len(prev), 100)] + ',,,' * (len(prev) > 100)}```
СТАЛО:
```
{now[:min(len(now), 100)] + ',,,' * (len(now) > 100)}```""", parse_mode = 'MarkdownV2', disable_web_page_preview=True)  
                cur.execute(f'''UPDATE sites
                            SET data = "{data_now}"
                            WHERE site = "{site}"''')
                con.commit()



class AddSite(StatesGroup):
    url = State()

@router.message(Command("add"))
async def start_handler(msg: types.Message, state: FSMContext):
    await state.set_state(AddSite.url)
    await msg.answer("Введите URL интересующего сайта 🌐")

@router.message(AddSite.url)
async def process_name(msg: types.Message, state: FSMContext):
    err = await add_url(msg.from_user.id, msg.text)
    if err == 2:
        await msg.answer("Успешно добавлен " + link('URL', msg.text) + ' 🔗', disable_web_page_preview=True, parse_mode = 'MarkdownV2')
    elif err == 0:
        await msg.answer("❗ Ошибка номер 35. Проверьте корректность url. ❗")
    else:
        await msg.answer("❗ Ошибка номер 73. Превышен лимит сайтов. ❗")
    await state.clear()




class DelSite(StatesGroup):
    num = State()

@router.message(Command("del"))
async def start_handler(msg: types.Message, state: FSMContext):
    await state.set_state(DelSite.num)
    sites = cur.execute("SELECT * FROM users WHERE user_id=?", (msg.from_user.id,)).fetchone()[1].strip().split(",")
    if sum([i.strip() != '' for i in sites]) == 0:
        await state.clear()
        await msg.answer("Нет добавленных сайтов 😢")
    else:
        for i in range(len(sites)):
            sites[i] = str(i + 1) + '. ' + sites[i]
        await msg.answer("Введите номер сайта из списка:\n" + '\n'.join(sites), disable_web_page_preview=True)

@router.message(DelSite.num)
async def process_name(msg: types.Message, state: FSMContext):
    if await del_url(msg.from_user.id, msg.text):
        await msg.answer("Успешно удалено ✔️")
    else:
        await msg.answer("❗ Ошибка номер 5. Проверьте корректность номера. ❗")
    await state.clear()




@router.message(Command("cancel"))
@router.message(F.text.casefold() == "cancel")
async def cancel_handler(message: Message, state: FSMContext) -> None:
    current_state = await state.get_state()
    if current_state is None:
        return
    logging.info("Cancelling state %r", current_state)
    await state.clear()
    await message.answer(
        "Cancelled.",
        reply_markup=ReplyKeyboardRemove(),
    )

async def main():
    global bot, dp
    bot = Bot(token='6848185044:AAFQK0_t8PKZrkS7m2dev1exppfG6dBXo7E', parse_mode=ParseMode.HTML)

    bot_commands = [
        BotCommand(command="/add", description="✏️ добавление URL"),
        BotCommand(command="/del", description="🗑️ удаление URL"),
        BotCommand(command="/nemalo", description="🚀 получение ссылки на канал Немало")
    ]
    await bot.set_my_commands(bot_commands)

    dp = Dispatcher()
    dp.include_router(router)
    scheduler.add_job(check_updates, "interval", minutes=10)
    scheduler.start()
    await dp.start_polling(bot)

if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, stream=sys.stdout)
    asyncio.run(main())
