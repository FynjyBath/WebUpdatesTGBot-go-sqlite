package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html"
)

var bot *tgbotapi.BotAPI
var err error
var DB *sql.DB
var MU sync.Mutex

type Site struct {
	site_id   string
	url       string
	data      string
	users_str string
}

func GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func RemoveNonUTF8Runes(s string) string {
	var valid []rune

	for i, w := 0, 0; i < len(s); i += w {
		runeValue, width := utf8.DecodeRuneInString(s[i:])
		if runeValue != utf8.RuneError {
			valid = append(valid, runeValue)
		}
		w = width
	}

	return string(valid)
}

func IsDateTime(str string) bool {
	timeFormats := []string{
		"2006-01-02",
		"2006/01/02",
		"02-Jan-2006",
		"Jan-02-2006",
		"02.01.2006",
		"2006.01.02",
		"01.02",
		"17:35",
		"15:04:05",
		"Apr/12/2024",
		"Apr/03/2024 14:54:27",
		"Feb/27/2024 17:35",
		"18.12.2006 15:50",
		"18.12.2006 15:50:50",
		"5:26",
	}
	for _, format := range timeFormats {
		if _, err := time.Parse(format, str); err == nil {
			return true
		}
	}
	return false
}

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func SendMessage(id int, message string) tgbotapi.Message {
	message = RemoveNonUTF8Runes(message)
	msg := tgbotapi.NewMessage(int64(id), message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg_sended, err := bot.Send(msg)
	if err != nil {
		panic(err)
	}
	return msg_sended
}

func GetData(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	html_text, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	data := ""

	domDocTest := html.NewTokenizer(strings.NewReader(string(html_text)))
	previousStartTokenTest := domDocTest.Token()

loop:
	for {
		tt := domDocTest.Next()
		switch {
		case tt == html.ErrorToken:
			break loop
		case tt == html.StartTagToken:
			previousStartTokenTest = domDocTest.Token()
		case tt == html.TextToken:
			if previousStartTokenTest.Data == "script" || previousStartTokenTest.Data == "style" {
				continue
			}
			TxtContent := strings.TrimSpace(html.UnescapeString(string(domDocTest.Text())))
			if len(TxtContent) > 0 {
				data += TxtContent + " "
			}
		}
	}

	return data, nil
}

func GetUpdate(data1, data2 string) (string, string) {
	pref := 0
	for pref < min(len(data1), len(data2)) && data1[pref] == data2[pref] {
		pref++
	}
	data1 = data1[pref:]
	data2 = data2[pref:]
	suf := 0
	for suf < min(len(data1), len(data2)) && data1[len(data1)-1-suf] == data2[len(data2)-1-suf] {
		suf++
	}
	return data1[:len(data1)-suf], data2[:len(data2)-suf]
}

func AddUrl(user_id int, url string) string {
	MU.Lock()
	defer MU.Unlock()

	var sites_str string
	err = DB.QueryRow("SELECT sites FROM users WHERE user_id = ?;", user_id).Scan(&sites_str)
	if err != nil {
		log.Fatal(err)
	}

	sites := strings.Split(sites_str, ",")
	if len(sites) == 1 && sites[0] == "" {
		sites = make([]string, 0)
	}
	if len(sites) == 10 {
		return "❗ Ошибка номер 73. Превышен лимит. ❗"
	}

	var site_id string
	err = DB.QueryRow("SELECT site_id FROM sites WHERE url = ?;", url).Scan(&site_id)
	if err != nil {
		site_id = GenerateID()
		data, err := GetData(url)
		if err != nil {
			return "❗ Ошибка номер 35. Проверьте корректность url. ❗"
		}

		_, err = DB.Exec("INSERT INTO sites VALUES (?, ?, ?, '');", site_id, url, data)
		if err != nil {
			log.Fatal(err)
		}
	}

	var users_str string
	err = DB.QueryRow("SELECT users FROM sites WHERE site_id = ?;", site_id).Scan(&users_str)
	if err != nil {
		log.Fatal(err)
	}

	users := strings.Split(users_str, ",")
	if len(users) == 1 && users[0] == "" {
		users = make([]string, 0)
	}
	users = append(users, fmt.Sprint(user_id))
	users_str = strings.Join(users, ",")
	_, err := DB.Exec("UPDATE sites SET users = ? WHERE site_id = ?;", users_str, site_id)
	if err != nil {
		log.Fatal(err)
	}

	sites = append(sites, site_id)
	sites_str = strings.Join(sites, ",")

	_, err = DB.Exec("UPDATE users SET sites = ? WHERE user_id = ?;", sites_str, user_id)
	if err != nil {
		log.Fatal(err)
	}

	return "Успешно добавлен [URL](" + url + ") 🔗"
}

func DelUrl(user_id, site_id int, url string) string {
	MU.Lock()
	defer MU.Unlock()

	var users_str string
	err = DB.QueryRow("SELECT users FROM sites WHERE site_id=?", site_id).Scan(&users_str)
	if err != nil {
		log.Fatal(err)
		return "❗ Произошла ошибка при получении списка пользователей для URL ❗"
	}

	users := strings.Split(users_str, ",")
	if len(users) == 1 && users[0] == "" {
		users = make([]string, 0)
	}
	for i, s := range users {
		if s == strconv.Itoa(user_id) {
			users = append(users[:i], users[i+1:]...)
			break
		}
	}

	users_str = strings.Join(users, ",")
	_, err = DB.Exec("UPDATE sites SET users = ? WHERE site_id = ?", users_str, site_id)
	if err != nil {
		log.Fatal(err)
		return "❗ Произошла ошибка при обновлении списка пользователей для URL ❗"
	}

	var sites_str string
	err = DB.QueryRow("SELECT sites FROM users WHERE user_id=?", user_id).Scan(&sites_str)
	if err != nil {
		log.Fatal(err)
		return "❗ Произошла ошибка при получении списка URL пользователя ❗"
	}

	sites := strings.Split(sites_str, ",")
	if len(sites) == 1 && sites[0] == "" {
		sites = make([]string, 0)
	}
	for i, s := range sites {
		if s == strconv.Itoa(site_id) {
			sites = append(sites[:i], sites[i+1:]...)
			break
		}
	}

	sites_str = strings.Join(sites, ",")
	_, err = DB.Exec("UPDATE users SET sites = ? WHERE user_id = ?", sites_str, user_id)
	if err != nil {
		log.Fatal(err)
		return "❗ Произошла ошибка при обновлении списка URL пользователя ❗"
	}

	return "[URL](" + url + ") успешно удален ✔️"
}

func CheckUpdates() {
	MU.Lock()
	rows, err := DB.Query("SELECT * FROM sites")
	if err != nil {
		return
	}
	defer rows.Close()
	MU.Unlock()

	var wg sync.WaitGroup
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.site_id, &site.url, &site.data, &site.users_str); err != nil {
			log.Fatal(err)
		}
		wg.Add(1)
		go func(site Site) {
			defer wg.Done()
			new_data, err := GetData(site.url)
			if err != nil {
				return
			}
			if site.data == new_data {
				return
			}

			before, after := GetUpdate(site.data, new_data)
			before = before[:min(len(before), 200)]
			if len(before) == 200 {
				before += ",,,"
			}
			after = after[:min(len(after), 200)]
			if len(after) == 200 {
				after += ",,,"
			}

			text := fmt.Sprintf("ИЗМЕНЕНИЕ НА: %s 🔗\n"+
				"БЫЛО:\n"+
				"```\n"+
				"%s```\n"+
				"СТАЛО:\n"+
				"```\n"+
				"%s```",
				"[URL]("+site.url+")", before, after)

			users := strings.Split(site.users_str, ",")
			if len(users) == 1 && users[0] == "" {
				users = make([]string, 0)
			}
			for i := 0; i < len(users); i++ {
				user_id, err := strconv.Atoi(users[i])
				if err != nil {
					log.Fatal(err)
				}
				SendMessage(user_id, text)
			}

			MU.Lock()
			_, err = DB.Exec("UPDATE sites SET data = ? WHERE site_id = ?", new_data, site.site_id)
			if err != nil {
				log.Fatal(err)
			}
			MU.Unlock()
		}(site)
	}
	wg.Wait()
}

func main() {
	DB, err = sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Panic(err)
		return
	}
	defer DB.Close()

	bot, err = tgbotapi.NewBotAPI("6803805831:AAFfMlkjOYipjorXm5ySAA11bmlg2vXcXeI")
	if err != nil {
		log.Panic(err)
		return
	}
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
		return
	}

	go func() {
		for {
			CheckUpdates()
			time.Sleep(time.Minute)
		}
	}()

	for update := range updates {
		if update.CallbackQuery != nil {
			user_id := update.CallbackQuery.From.ID
			var exists bool
			err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ?);", user_id).Scan(&exists)
			if err != nil {
				log.Fatal(err)
			}
			if !exists {
				_, err := DB.Exec("INSERT INTO users(user_id, sites) VALUES (?, '');", user_id)
				if err != nil {
					log.Fatal(err)
				}
			}

			site_id, err := strconv.Atoi(update.CallbackQuery.Data)
			if err != nil {
				log.Fatal(err)
			}
			var url string
			err = DB.QueryRow("SELECT url FROM sites WHERE site_id=?", site_id).Scan(&url)
			if err != nil {
				log.Fatal(err)
			}
			SendMessage(user_id, DelUrl(user_id, site_id, url))

			var sites_str string
			err = DB.QueryRow("SELECT sites FROM users WHERE user_id=?", user_id).Scan(&sites_str)
			if err != nil {
				log.Fatal(err)
			}
			sites := strings.Split(sites_str, ",")
			if len(sites) == 1 && sites[0] == "" {
				sites = make([]string, 0)
			}

			if len(sites) == 0 {
				editedMessageText := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "Нет добавленных сайтов 😢")
				_, err = bot.Send(editedMessageText)
				if err != nil {
					log.Println(err)
				}

				keyboard := tgbotapi.NewInlineKeyboardMarkup()
				editedMessageMarkup := tgbotapi.NewEditMessageReplyMarkup(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, keyboard)
				_, err = bot.Send(editedMessageMarkup)
				if err != nil {
					log.Println(err)
				}
			} else {
				var rows [][]tgbotapi.InlineKeyboardButton
				for _, site_id := range sites {
					var url string
					err = DB.QueryRow("SELECT url FROM sites WHERE site_id=?", site_id).Scan(&url)
					if err != nil {
						log.Fatal(err)
					}

					btn := tgbotapi.NewInlineKeyboardButtonData(url, site_id)
					row := tgbotapi.NewInlineKeyboardRow(btn)
					rows = append(rows, row)
				}

				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

				editedMessage := tgbotapi.NewEditMessageReplyMarkup(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, keyboard)
				_, err = bot.Send(editedMessage)
				if err != nil {
					log.Println(err)
				}
			}
		} else if update.Message != nil {
			user_id := update.Message.From.ID
			var exists bool
			err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ?);", user_id).Scan(&exists)
			if err != nil {
				log.Fatal(err)
			}
			if !exists {
				_, err := DB.Exec("INSERT INTO users(user_id, sites) VALUES (?, '');", user_id)
				if err != nil {
					log.Fatal(err)
				}
			}
			if update.Message.IsCommand() {
				command := update.Message.Command()
				switch command {
				case "start":
					SendMessage(user_id, "Привет!\nСписок доступных команд:\n✏️ /add - добавление URL\n🗑️ /del - удаление URL")
				case "add":
					url := update.Message.CommandArguments()
					SendMessage(user_id, AddUrl(user_id, url))
				case "del":
					var sites_str string
					err = DB.QueryRow("SELECT sites FROM users WHERE user_id=?", user_id).Scan(&sites_str)
					if err != nil {
						log.Fatal(err)
					}
					sites := strings.Split(sites_str, ",")
					if len(sites) == 1 && sites[0] == "" {
						sites = make([]string, 0)
					}

					if len(sites) == 0 {
						SendMessage(user_id, "Нет добавленных сайтов 😢")
					} else {
						var rows [][]tgbotapi.InlineKeyboardButton
						for _, site_id := range sites {
							var url string
							err = DB.QueryRow("SELECT url FROM sites WHERE site_id=?", site_id).Scan(&url)
							if err != nil {
								log.Fatal(err)
							}

							btn := tgbotapi.NewInlineKeyboardButtonData(url, site_id)
							row := tgbotapi.NewInlineKeyboardRow(btn)
							rows = append(rows, row)
						}

						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нажмите на кнопку для удаления:")
						keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
						msg.ReplyMarkup = keyboard

						_, err = bot.Send(msg)
						if err != nil {
							panic(err)
						}
					}
				}
			}
		}
	}
}
