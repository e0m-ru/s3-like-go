package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

// curl --data-binary @/path/to/your/file --url https://localhost/upload/file

const (
	STORAGE_DIR         = "/storage"        // ДИРЕКТОРИЯ ДЛЯ ХРАНЕНИЯ ОБЪЕКТОВ
	UPLOAD_PREFIX_LEN   = len("/upload/")   // ДЛИНА ПРЕФИКСА ДЛЯ МАРШРУТА ЗАГРУЗКИ
	DOWNLOAD_PREFIX_LEN = len("/download/") // ДЛИНА ПРЕФИКСА ДЛЯ МАРШРУТА ЗАГРУЗКИ
)

// Storage — структура для хранения объектов в памяти
type Storage struct {
	mu    sync.RWMutex   // Мьютекс для обеспечения потокобезопасности
	files map[string]obj // Хэш-таблица для хранения данных объектов
}

// NewStorage — конструктор для создания нового хранилища
func NewStorage() *Storage {
	return &Storage{
		files: make(map[string]obj),
	}
}

// Save — метод для сохранения объекта в хранилище
func (s *Storage) Save(key string, data []byte) error {
	s.mu.Lock()         // Захватываем мьютекс перед записью
	defer s.mu.Unlock() // Освобождаем мьютекс после записи
	if _, exists := s.files[key]; exists {
		return fmt.Errorf("object %v already exists", key)
	}
	// Сохраняем данные в памяти
	s.files[key] = obj{name: key, body: data}

	// Также сохраняем данные на диск
	err := os.WriteFile(STORAGE_DIR+"/"+key, data, 0644)
	if err != nil {
		log.Printf("Ошибка при сохранении файла %s: %v", key, err)
		return err
	}

	return nil
}

// Load — метод для загрузки объекта из хранилища
func (s *Storage) Load(key string) (obj, bool) {
	s.mu.Lock()         // Захватываем мьютекс перед чтением
	defer s.mu.Unlock() // Освобождаем мьютекс после чтения

	// Проверяем наличие объекта в памяти
	data, exists := s.files[key]
	if exists {
		return data, true
	}

	// Если объект не найден в памяти, пытаемся загрузить его с диска
	file, err := os.ReadFile(STORAGE_DIR + "/" + key)
	if err != nil {
		return obj{}, false
	}

	// Если загрузка с диска успешна, кэшируем объект в памяти
	s.files[key] = obj{name: key, body: file}
	return data, true
}

// Объект в хранилище
type obj struct {
	name string
	body []byte
}

// HandleUpload — обработчик для загрузки объектов
func HandleUpload(w http.ResponseWriter, r *http.Request, storage *Storage) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ключ (имя объекта) из URL
	key := r.URL.Path[UPLOAD_PREFIX_LEN:]

	// Читаем тело запроса (данные объекта)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Сохраняем объект в хранилище
	err = storage.Save(key, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
	} else {
		// Отправляем ответ клиенту
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Объект %s успешно сохранен", key)
	}

}

// HandleDownload — обработчик для загрузки объектов
func HandleDownload(w http.ResponseWriter, r *http.Request, storage *Storage) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ключ (имя объекта) из URL
	key := r.URL.Path[DOWNLOAD_PREFIX_LEN:]

	// Загружаем объект из хранилища
	data, exists := storage.Load(key)
	if !exists {
		http.Error(w, "Объект не найден", http.StatusNotFound)
		return
	}

	// Отправляем данные объекта клиенту
	w.WriteHeader(http.StatusOK)
	w.Write(data.body)
}

// HandleList — обработчик для вывода списка всех объектов
func HandleList(w http.ResponseWriter, r *http.Request, storage *Storage) {
	type List struct {
		Name   string
		InCach bool
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Захватываем мьютекс для доступа к хэш-таблице объектов
	storage.mu.Lock()
	defer storage.mu.Unlock()

	// Создаем список ключей (имен объектов)
	files, err := os.ReadDir(STORAGE_DIR)
	if err != nil {
		log.Panicf("Не получилось прочитать дерикторию %v: %v", STORAGE_DIR, err)
	}

	keys := make([]List, 0, len(files))

	for key := range storage.files {
		keys = append(keys, List{key, true})
	}

	for _, f := range files {
		if _, exist := storage.files[f.Name()]; !exist {
			keys = append(keys, List{f.Name(), false})
		}
	}

	// Кодируем список ключей в формат JSON и отправляем клиенту
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

func main() {
	// Проверяем наличие директории для хранения объектов
	if _, err := os.Stat(STORAGE_DIR); os.IsNotExist(err) {
		err := os.Mkdir(STORAGE_DIR, 0755)
		if err != nil {
			log.Fatalf("Ошибка создания директории %s: %v", STORAGE_DIR, err)
		}
	}

	// Создаем новое хранилище
	storage := NewStorage()

	// Настраиваем маршруты для обработки HTTP-запросов
	http.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		HandleUpload(w, r, storage)
	})
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		HandleDownload(w, r, storage)
	})
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		HandleList(w, r, storage)
	})

	// Запускаем HTTP-сервер на порту 8080
	log.Println("Сервер запущен на порту 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
