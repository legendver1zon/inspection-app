package handlers

import (
	"fmt"
	"time"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"
)

// actCard — осмотр с агрегатами для карточки/строки списка.
type actCard struct {
	models.Inspection
	HumanDate   string
	Defects     int64
	Photos      int64
	FilledRooms int64
	TotalRooms  int64
	Percent     int
	CloudState  string // "" | "queue" | "error" | "ok"
	CloudN      int64
}

var ruMonthsGen = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d %s", t.Day(), ruMonthsGen[t.Month()-1])
}

// buildActCards дополняет осмотры агрегатами: помещения (всего/заполнено),
// дефекты, фото и статус выгрузки в облако. Четыре GROUP BY-запроса на весь
// список — без N+1.
func buildActCards(list []models.Inspection) []actCard {
	cards := make([]actCard, len(list))
	ids := make([]uint, len(list))
	for i := range list {
		ids[i] = list[i].ID
		cards[i] = actCard{Inspection: list[i], HumanDate: humanDate(list[i].Date)}
	}
	if len(ids) == 0 {
		return cards
	}

	type idCount struct {
		InspectionID uint
		C            int64
	}

	scanMap := func(rows []idCount) map[uint]int64 {
		m := make(map[uint]int64, len(rows))
		for _, r := range rows {
			m[r.InspectionID] = r.C
		}
		return m
	}

	var rows []idCount
	storage.DB.Table("inspection_rooms").
		Select("inspection_id, count(*) as c").
		Where("inspection_id IN ? AND deleted_at IS NULL", ids).
		Group("inspection_id").Scan(&rows)
	totalRooms := scanMap(rows)

	rows = nil
	storage.DB.Table("inspection_rooms ir").
		Select("ir.inspection_id as inspection_id, count(distinct ir.id) as c").
		Joins("JOIN room_defects rd ON rd.room_id = ir.id AND rd.deleted_at IS NULL").
		Where("ir.inspection_id IN ? AND ir.deleted_at IS NULL", ids).
		Group("ir.inspection_id").Scan(&rows)
	filledRooms := scanMap(rows)

	rows = nil
	storage.DB.Table("room_defects rd").
		Select("ir.inspection_id as inspection_id, count(*) as c").
		Joins("JOIN inspection_rooms ir ON ir.id = rd.room_id").
		Where("ir.inspection_id IN ? AND rd.deleted_at IS NULL AND ir.deleted_at IS NULL", ids).
		Group("ir.inspection_id").Scan(&rows)
	defects := scanMap(rows)

	// Фото считаем включая архивные дефекты/помещения: они показываются
	// на странице просмотра и выгружаются в облако наравне с остальными
	type idStatusCount struct {
		InspectionID uint
		UploadStatus string
		C            int64
	}
	var srows []idStatusCount
	storage.DB.Table("photos p").
		Select("p.inspection_id as inspection_id, p.upload_status, count(*) as c").
		Where("p.inspection_id IN ? AND p.deleted_at IS NULL", ids).
		Group("p.inspection_id, p.upload_status").Scan(&srows)

	photoTotal := map[uint]int64{}
	photoFailed := map[uint]int64{}
	photoActive := map[uint]int64{} // pending + uploading
	for _, r := range srows {
		photoTotal[r.InspectionID] += r.C
		switch r.UploadStatus {
		case "failed":
			photoFailed[r.InspectionID] += r.C
		case "pending", "uploading":
			photoActive[r.InspectionID] += r.C
		}
	}

	for i := range cards {
		id := cards[i].ID
		cards[i].TotalRooms = totalRooms[id]
		cards[i].FilledRooms = filledRooms[id]
		cards[i].Defects = defects[id]
		cards[i].Photos = photoTotal[id]
		if cards[i].TotalRooms > 0 {
			cards[i].Percent = int(cards[i].FilledRooms * 100 / cards[i].TotalRooms)
		}
		switch {
		case photoFailed[id] > 0:
			cards[i].CloudState, cards[i].CloudN = "error", photoFailed[id]
		case photoActive[id] > 0:
			cards[i].CloudState, cards[i].CloudN = "queue", photoActive[id]
		case photoTotal[id] > 0:
			cards[i].CloudState = "ok"
		}
	}
	return cards
}
