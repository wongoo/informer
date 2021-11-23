package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	configFileName         = "foodorder.json"
	previousChosenFileName = "previous_chosen.json"
)

// Menu 菜单
type Menu struct {
	Type     string   `json:"type"`
	ChoseNum int      `json:"chose_num"`
	List     []string `json:"list"`
}

// Config 食物配置
type Config struct {
	Partners    []string      `json:"partners"`
	Restaurants []*Restaurant `json:"restaurants"`
}

// Restaurant 餐厅
type Restaurant struct {
	Name  string  `json:"name"`
	Tel   string  `json:"tel"`
	Menus []*Menu `json:"food_list"`
}

// Order 下单
type Order struct {
	RestaurantName string              `json:"name"`
	User           string              `json:"user"`
	Chose          map[string][]string `json:"chose"`
}

func main() {
	buf := bytes.NewBuffer(nil)

	now := time.Now()
	weekday := time.Now().Weekday()

	dateString := fmt.Sprintf("今天是 %s %s\n\n", now.Format("2006-01-02"), weekday.String())
	buf.WriteString(dateString)

	if dailySoup := GetDailySoup(); dailySoup != "" {
		buf.WriteString(dailySoup)
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	dataPath := filepath.Join(exeDir, configFileName)
	data, err := ioutil.ReadFile(dataPath)
	if err != nil {
		log.Fatal(err)
	}

	var foodConfig Config
	if err := json.Unmarshal(data, &foodConfig); err != nil {
		log.Fatal(err)
	}

	var previousFoodOrders []*Order
	previousData, err := ioutil.ReadFile(filepath.Join(exeDir, previousChosenFileName))
	if err != nil {
		log.Printf("read previous chosen error: %v", err)
	}

	_ = json.Unmarshal(previousData, &previousFoodOrders)

	var orderUser string
	if len(foodConfig.Partners) > 0 {
		orderUser = foodConfig.Partners[0]
	}
	if len(previousFoodOrders) > 0 {
		previousLatestOrder := previousFoodOrders[len(previousFoodOrders)-1]
		if previousLatestOrder.User != "" {
			for i, u := range foodConfig.Partners {
				if u == previousLatestOrder.User && i < len(foodConfig.Partners)-1 {
					orderUser = foodConfig.Partners[i+1]
				}
			}
		}
	}

	fmt.Printf("order user: %s\n", orderUser)

	if weekday == time.Sunday || weekday == time.Saturday {
		buf.WriteString("周末愉快!")
	} else {
		autoChoseFood(buf, exeDir, &foodConfig, previousFoodOrders, &orderUser)
	}

	content := string(buf.Bytes())
	fmt.Print(content)

	if len(os.Args) > 1 {
		// ding(os.Args[1], content, orderUser, weekday)
		lark(os.Args[1], content, orderUser, weekday)
	}
}

func autoChoseFood(buf *bytes.Buffer, exeDir string, foodConfig *Config, previousFoodOrders []*Order, orderUser *string) {
	rand.Seed(time.Now().Unix())

	if len(previousFoodOrders) == len(foodConfig.Restaurants) {
		previousFoodOrders = nil
	}

	restaurants := filterPreviousChosenRestaurants(previousFoodOrders, foodConfig.Restaurants)

	restaurantIndex := rand.Intn(len(restaurants))
	restaurant := restaurants[restaurantIndex]

	buf.WriteString("上班辛苦了! 中午为你推荐餐厅《" + restaurant.Name + "》")
	if restaurant.Tel != "" {
		buf.WriteString("(点餐电话" + restaurant.Tel + ")")
	}

	rand.Seed(time.Now().Unix())

	foodOrder := &Order{
		RestaurantName: restaurant.Name,
		User:           *orderUser,
		Chose:          make(map[string][]string),
	}
	buf.WriteString("\n\n")

	if len(restaurant.Menus) == 0 {
		*orderUser = ""
	} else {
		for _, foodMenu := range restaurant.Menus {
			buf.WriteString(foodMenu.Type)
			buf.WriteByte(':')
			for i := 0; i < foodMenu.ChoseNum; i++ {
				index := rand.Intn(len(foodMenu.List))
				if i > 0 {
					buf.WriteByte(',')
				}
				buf.WriteString(foodMenu.List[index])

				foodOrder.Chose[foodMenu.Type] = append(foodOrder.Chose[foodMenu.Type], foodMenu.List[index])
				foodMenu.List = append(foodMenu.List[:index], foodMenu.List[index+1:]...)
			}
			buf.WriteByte('\n')
		}

		if restaurant.Tel != "" {
			buf.WriteString("\n需要一起点餐的同学+1, 被@的同学负责点餐~\n")
		}
	}

	previousFoodOrders = append(previousFoodOrders, foodOrder)
	if b, err := json.Marshal(previousFoodOrders); err == nil {
		_ = ioutil.WriteFile(filepath.Join(exeDir, previousChosenFileName), b, 0660)
	}
}

func filterPreviousChosenRestaurants(previousFoodOrders []*Order, restaurants []*Restaurant) []*Restaurant {
	if len(previousFoodOrders) == 0 {
		return restaurants
	}

	var results []*Restaurant
LOOP1:
	for _, r := range restaurants {
		for _, t := range previousFoodOrders {
			if t.RestaurantName == r.Name {
				continue LOOP1
			}
		}
		results = append(results, r)
	}

	if len(results) == 0 {
		return restaurants
	}

	return results
}

func filterPreviousChosenMenus(previousFoodOrders []*Order, restaurant *Restaurant) {
	var previousFoodOrder *Order
	for _, o := range previousFoodOrders {
		if o.RestaurantName == restaurant.Name {
			previousFoodOrder = o
			break
		}
	}

	if previousFoodOrder == nil || len(previousFoodOrder.Chose) == 0 {
		return
	}

	log.Printf("previous chosen: %v", previousFoodOrder.Chose)

	for key := range previousFoodOrder.Chose {
		for _, foodMenu := range restaurant.Menus {
			if foodMenu.Type == key {
				filterItems(foodMenu, previousFoodOrder.Chose[key])
				break
			}
		}
	}
}

func filterItems(menu *Menu, filters []string) {
	for _, filter := range filters {
		for index, name := range menu.List {
			if name == filter {
				menu.List = append(menu.List[:index], menu.List[index+1:]...)
				break
			}
		}
	}

	log.Printf("after filter for %s: %s", menu.Type, menu.List)
}


func GetDailySoup() string {
	resp, err := http.Get("http://open.iciba.com/dsapi/")
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return ""
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return ""
	}
	if resp.StatusCode != 200 {
		fmt.Printf("err: %d, %s\n", resp.StatusCode, b)
		return ""
	}

	data := struct {
		Content string
	}{}

	if err = json.Unmarshal(b, &data); err != nil {
		fmt.Printf("err: %v\n", err)
		return ""
	}

	return data.Content
}
