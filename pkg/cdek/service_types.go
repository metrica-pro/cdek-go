package cdek

// service_types.go - Удобные типы для высокоуровневого Service API (PascalCase по регламенту)
//
// Эти типы являются обертками над автогенерированными CDEK типами,
// упрощающими использование библиотеки в приложениях.

// ========================
// Cost Calculation
// ========================

// CostRequest - запрос на расчет стоимости доставки
type CostRequest struct {
	FromCityCode int32     // Код города отправителя (по КЛАДР)
	ToCityCode   int32     // Код города получателя (по КЛАДР)
	Packages     []Package // Список мест (упаковок)
}

// Package - информация об одной упаковке (месте)
type Package struct {
	Weight int32 // Вес в граммах
	Length int32 // Длина в см
	Width  int32 // Ширина в см
	Height int32 // Высота в см
}

// CostResponse - ответ с вариантами доставки и стоимостью
type CostResponse struct {
	Tariffs []TariffOption // Доступные тарифы
}

// TariffOption - один вариант тарифа доставки
type TariffOption struct {
	TariffCode   int     // Код тарифа
	TariffName   string  // Название тарифа
	DeliveryMode int     // Режим доставки: 1=дверь-дверь, 2=дверь-склад, 3=склад-дверь, 4=склад-склад
	DeliverySum  float64 // Стоимость доставки (рубли)
	PeriodMin    int     // Минимальный срок доставки (дней)
	PeriodMax    int     // Максимальный срок доставки (дней)
}

// ========================
// Orders
// ========================

// OrderRequest - запрос на создание заказа
type OrderRequest struct {
	Type         string        // Тип заказа: "delivery" (доставка), "pickup" (самовывоз)
	TariffCode   int           // Код тарифа
	Comment      *string       // Комментарий к заказу
	Sender       Contact       // Отправитель
	Recipient    Recipient     // Получатель
	FromLocation Location      // Адрес отправителя
	ToLocation   Location      // Адрес получателя
	Packages     []OrderPackage // Список мест
}

// Contact - контактная информация
type Contact struct {
	Company *string // Название компании
	Name    string  // ФИО контактного лица
	Email   *string // Email
	Phones  []Phone // Список телефонов
}

// Recipient - информация о получателе (расширяет Contact)
type Recipient struct {
	Contact                 // Базовая контактная информация
	PassportSeries          *string // Серия паспорта
	PassportNumber          *string // Номер паспорта
	PassportDateOfIssue     *string // Дата выдачи паспорта
	PassportOrganization    *string // Кем выдан паспорт
}

// Phone - телефонный номер
type Phone struct {
	Number     string  // Номер телефона (обязательно)
	Additional *string // Добавочный номер
}

// Location - местоположение (адрес)
type Location struct {
	Code       *int32  // Код населенного пункта СДЭК
	FiasGuid   *string // Уникальный идентификатор ФИАС
	PostalCode *string // Почтовый индекс
	CountryCode *string // Код страны (ISO 3166-1 alpha-2)
	Region     *string // Регион
	City       *string // Город
	Address    *string // Адрес (улица, дом, квартира)
}

// OrderPackage - информация об упаковке в заказе
type OrderPackage struct {
	Number  string  // Номер упаковки (артикул)
	Weight  int32   // Общий вес (граммы)
	Length  *int32  // Длина (см)
	Width   *int32  // Ширина (см)
	Height  *int32  // Высота (см)
	Comment *string // Комментарий
	Items   []Item  // Список вложений
}

// Item - вложение в упаковку
type Item struct {
	Name    string  // Наименование товара
	WareKey string  // Артикул товара
	Payment float64 // Оплата (за единицу товара, в т.ч. частичная предоплата)
	Cost    float64 // Объявленная стоимость товара (за единицу)
	Weight  int32   // Вес (граммы, за единицу)
	Amount  int32   // Количество единиц товара
}

// OrderResponse - ответ при создании заказа
type OrderResponse struct {
	UUID       string        // Идентификатор заказа в CDEK
	Number     *string       // Номер заказа CDEK (может быть null до обработки)
	TariffCode int           // Код тарифа
	Statuses   []StatusEvent // История статусов
	CreatedAt  string        // Дата и время создания (ISO 8601)
}

// StatusEvent - событие изменения статуса
type StatusEvent struct {
	Code     string  // Код статуса
	Name     string  // Название статуса
	DateTime string  // Дата и время статуса (ISO 8601)
	City     *string // Город, в котором произошло событие
}

// UpdateOrderRequest - запрос на обновление заказа
type UpdateOrderRequest struct {
	OrderUUID    string         // UUID заказа для обновления
	Recipient    *Recipient     // Новые данные получателя
	Sender       *Contact       // Новые данные отправителя
	ToLocation   *Location      // Новый адрес доставки
	FromLocation *Location      // Новый адрес отправления
	Comment      *string        // Новый комментарий
	Packages     []OrderPackage // Обновленный список мест (если нужно)
}

// OrderInfo - полная информация о заказе
type OrderInfo struct {
	UUID              string         // Идентификатор заказа в CDEK
	Number            *string        // Номер заказа CDEK
	Type              string         // Тип заказа
	TariffCode        int            // Код тарифа
	Sender            Contact        // Отправитель
	Recipient         Recipient      // Получатель
	FromLocation      Location       // Адрес отправления
	ToLocation        Location       // Адрес доставки
	Packages          []OrderPackage // Список мест
	Statuses          []StatusEvent  // История статусов
	CreatedAt         string         // Дата создания
	DeliveryCost      *float64       // Стоимость доставки
	EstimatedDelivery *string        // Планируемая дата доставки
	ActualDelivery    *string        // Фактическая дата доставки
}

// ========================
// Tracking
// ========================

// TrackingInfo - информация об отслеживании заказа
type TrackingInfo struct {
	UUID              string        // Идентификатор заказа в CDEK
	Number            *string       // Номер заказа CDEK
	CurrentStatus     StatusEvent   // Текущий статус
	StatusHistory     []StatusEvent // История статусов
	EstimatedDelivery *string       // Планируемая дата доставки (ISO 8601)
	ActualDelivery    *string       // Фактическая дата доставки (ISO 8601)
}

// ========================
// Delivery Points
// ========================

// DeliveryPointsRequest - запрос на получение списка ПВЗ
type DeliveryPointsRequest struct {
	CityCode string // Код города (по КЛАДР)
	Type     string // Тип пункта: "PVZ" (пункт выдачи), "POSTAMAT" (постамат)
}

// DeliveryPoint - пункт выдачи заказов
type DeliveryPoint struct {
	Code        string        // Код ПВЗ
	Name        string        // Название ПВЗ
	Type        string        // Тип: "PVZ", "POSTAMAT"
	Location    PointLocation // Адрес расположения
	WorkTime    string        // Режим работы
	Phones      []Phone       // Телефоны
	Email       *string       // Email
	Note        *string       // Примечание
	OfficeImage *string       // URL изображения офиса
}

// PointLocation - местоположение пункта выдачи
type PointLocation struct {
	Country    string  // Страна
	Region     string  // Регион
	City       string  // Город
	Address    string  // Адрес
	PostalCode string  // Почтовый индекс
	Latitude   float64 // Широта
	Longitude  float64 // Долгота
}

// ========================
// Print (Barcode/Waybill)
// ========================

// PrintBarcodeRequest - запрос на создание этикеток
type PrintBarcodeRequest struct {
	Orders []PrintOrder // Список заказов для печати
	Copy   *int         // Количество копий (по умолчанию 1)
	Format *string      // Формат: "A4", "A5", "A6" (по умолчанию A4)
}

// PrintWaybillRequest - запрос на создание накладных
type PrintWaybillRequest struct {
	Orders []PrintOrder // Список заказов для печати
	Copy   *int         // Количество копий (по умолчанию 1)
	Format *string      // Формат: "A4", "A5" (по умолчанию A4)
}

// PrintOrder - заказ для печати
type PrintOrder struct {
	OrderUUID string // UUID заказа
}

// PrintResponse - ответ на запрос печати
type PrintResponse struct {
	UUID      string // UUID задания на печать
	URL       string // URL для скачивания PDF (доступен после готовности)
	Status    string // Статус: "ACCEPTED", "PROCESSING", "READY", "INVALID"
	CreatedAt string // Дата создания задания
}
