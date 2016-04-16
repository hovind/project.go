package main

import (
    "encoding/json"
    "fmt"
    "time"
    //"os"

    "project.go/elev"
    "project.go/network"
    . "project.go/obj"
    "project.go/order"
    "project.go/timer"
)

const (
    N_FLOORS  = 4
    N_BUTTONS = 3
)
func network_decoder(from_network_channel <-chan Message) (<-chan struct{o Order; a string}, <-chan struct{s order.Orders; a string}, <-chan struct{v int; a string}, <-chan struct{v int; a string}) {
    order_from_network_channel := make(chan struct{o Order; a string});
    sync_from_network_channel := make(chan struct{s order.Orders; a string});
    floor_from_network_channel := make(chan struct{v int; a string});
    direction_from_network_channel := make(chan struct{v int; a string});
    go func() {
        for {
            msg := <-from_network_channel;
            addr := msg.Origin.IP.String();

            v, o, s, err := 0, Order{}, order.Orders{}, error(nil)
            switch msg.Code {
            case ORDER:
                err = json.Unmarshal(msg.Body, &o)
            case FLOOR_UPDATE, DIRECTION_UPDATE:
                err = json.Unmarshal(msg.Body, &v)
            case SYNC:
                err = json.Unmarshal(msg.Body, &s);
            }
            if err != nil {
                fmt.Println("Could not unmarshal order.")
                continue;
            }
            switch msg.Code {
            case ORDER:
                data := struct{o Order; a string}{o, addr}
                order_from_network_channel <-data;
            case FLOOR_UPDATE:
                data := struct{v int; a string}{v, addr}
                floor_from_network_channel <-data;
            case DIRECTION_UPDATE:
                data := struct{v int; a string}{v, addr}
                direction_from_network_channel <-data;
            case SYNC:
                data := struct{s order.Orders; a string}{s, addr};
                sync_from_network_channel <-data;
            }
        }
    }();
    return order_from_network_channel, sync_from_network_channel, floor_from_network_channel, direction_from_network_channel;
}

func network_encoder(to_network_channel chan<- Message) (chan<- Order, chan<- order.Orders, chan<- int, chan<- int) {
    order_to_network_channel := make(chan Order);
    sync_to_network_channel := make(chan order.Orders);
    floor_to_network_channel := make(chan int);
    direction_to_network_channel := make(chan int);
    go func() {
        for {
            select {
            case order := <- order_to_network_channel:
                b, err := json.Marshal(order);
                if err != nil {
                    fmt.Println("Could not marshal order.");
                } else {
                    to_network_channel <-*NewMessage(ORDER, b, nil, nil);
                }
            case orders := <-sync_to_network_channel:
                b, err := json.Marshal(orders);
                if err != nil {
                    fmt.Println("Could not marshal order.");
                } else {
                    to_network_channel <-*NewMessage(SYNC, b, nil, nil);
                }
            case floor := <-floor_to_network_channel:
                b, err := json.Marshal(floor);
                if err != nil {
                    fmt.Println("Could not marshal floor.");
                } else {
                    to_network_channel <-*NewMessage(FLOOR_UPDATE, b, nil, nil);
                }
            case direction := <-direction_to_network_channel:
                b, err := json.Marshal(direction);
                if err != nil {
                    fmt.Println("Could not marshal direction.");
                } else {
                    to_network_channel <-*NewMessage(DIRECTION_UPDATE, b, nil, nil);
                }
            }
        }
    }();
    return order_to_network_channel, sync_to_network_channel, floor_to_network_channel, direction_to_network_channel;
}

func order_manager(light_channel chan<- Order) (chan<- Order, chan<- int, chan chan int, chan chan int, chan chan int) {
    local_addr, to_network_channel, from_network_channel := network.Manager("33223")



    order_to_network_channel,
    sync_to_network_channel,
    floor_to_network_channel,
    direction_to_network_channel := network_encoder(to_network_channel);

    order_from_network_channel,
    sync_from_network_channel,
    floor_from_network_channel,
    direction_from_network_channel := network_decoder(from_network_channel);

    order_channel := make(chan Order);
    floor_channel := make(chan int);
    stop_request_channel := make(chan chan int);
    direction_request_channel := make(chan chan int);
    order_request_channel := make(chan chan int);

    system := order.NewOrders(local_addr)
    go func() {
        floor := -1;
        new_order := false;
        for {
            //system.Print();
            select {
            case data := <-order_from_network_channel:
                if !system.CheckIfCart(data.a) {
                    system.AddCartToMap(order.NewCart(), data.a)
                }
                if data.o.Button == order.COMMAND {
                    if data.a == local_addr {
                        light_channel <-data.o;
                    }
                    system.SetCommand(data.a, data.o.Floor, data.o.Value)
                } else {
                    light_channel <-data.o;
                    system.SetHallOrder(data.o.Floor, data.o.Button, data.o.Value)
                    //hall[order.Floor][order.Button] = value;
                }
                if data.o.Value {
                    new_order = true;
                }
            case data := <-sync_from_network_channel:
                fmt.Println("SYNC OBJECT:", data.s)
                fmt.Println(data.s.Addr, "vs", local_addr)
                if data.s.Addr == local_addr {
                    system.Sync(&data.s, light_channel);
                } else {
                    data.s.Sync(system, light_channel);
                    sync_to_network_channel <-data.s;
                }
            case data := <-floor_from_network_channel:
                if !system.CheckIfCart(data.a) {
                    system.AddCartToMap(order.NewCart(), data.a)
                }
                system.SetFloor(data.a, data.v)
            case data := <-direction_from_network_channel:
                if !system.CheckIfCart(data.a) {
                    system.AddCartToMap(order.NewCart(), data.a)
                }
                system.SetDir(data.a, data.v)
            case order := <-order_channel:
                if order.Button == 2/*order.COMMAND*/ {
                    system.SetCommand(local_addr, order.Floor, order.Value)
                } else {
                    system.SetHallOrder(order.Floor, order.Button, order.Value)
                    //hall[order.Floor][order.Button] = value;
                }
                if order.Value {
                    new_order = true;
                }
                light_channel <-order;
                order_to_network_channel <-order;
            case floor = <-floor_channel:
                fmt.Println("HEELO");
                system.SetFloor(local_addr, floor);
                  fmt.Println("HESELO");
                //carts[local_addr].Floor = floor;
                floor_to_network_channel <-floor;
            case response_channel := <-stop_request_channel:
                fmt.Println("Floor:", floor, "Direction:", system.CurDir(local_addr))
                floor_action := system.CheckFloorAction(floor, system.CurDir(local_addr));
                if floor_action == order.OPEN_DOOR {
                    order_to_network_channel <-Order{Button: order.COMMAND, Floor: floor, Value: false}
                }
                response_channel <-floor_action;
            case response_channel := <-direction_request_channel:
                direction := system.GetDirection()
                system.SetDir(local_addr, direction)
                direction_to_network_channel <-direction;

                button := order.UP
                floor := system.CurFloor(local_addr)
                if direction == elev.DOWN {
                    button = order.DOWN
                } else if direction == elev.STOP {
                    order_to_network_channel <-Order{Button: order.DOWN, Floor: floor, Value: false}
                }
                order_to_network_channel <-Order{Button: button, Floor: floor, Value: false}
                //carts[local_addr].Direction = direction;
                response_channel <- direction
            case response_channel := <-order_request_channel:
                if new_order {
                    response_channel <-1;
                    new_order = false;
                } else {
                    response_channel <-0;
                }
            }
        }
    }()
    return order_channel, floor_channel, stop_request_channel, direction_request_channel, order_request_channel;
}

func light_manager() chan<- Order {
    light_channel := make(chan Order);

    go func() {
        for {
            order := <-light_channel;
            elev.SetButtonLamp(order.Button, order.Floor, order.Value);
        }
    }();
    return light_channel;
}

func main() {
    door_open := false;
    door_timer := timer.New();

    elev.Init();
    elev.SetMotorDirection(elev.DOWN);

    button_channel := elev.Button_checker();
    floor_sensor_channel := elev.Floor_checker();
    stop_button_channel := elev.Stop_checker();

    light_channel := light_manager();
    order_channel, floor_channel, stop_request_channel, direction_request_channel, order_request_channel := order_manager(light_channel);

    floor := -1;
    direction := elev.DOWN;
    for {
        select {
        case order := <-button_channel:
            if floor == order.Floor && door_open {
                door_timer.Start(3*time.Second);
            } else {
                order_channel <-order;
            }
        case floor = <-floor_sensor_channel:
            elev.SetFloorIndicator(floor);
            floor_channel <-floor;
            floor_action := request(stop_request_channel);
            if floor_action == order.OPEN_DOOR {
                open_door(door_timer, &door_open);
            } else if floor_action == order.STOP {
                elev.SetMotorDirection(elev.STOP);
            }
            direction = elev.STOP;
        case <-stop_button_channel:
            elev.SetMotorDirection(elev.STOP);
        case <-door_timer.Timer.C:
            door_open = false;
            elev.SetDoorOpenLamp(false);
            direction = request(direction_request_channel);
            elev.SetMotorDirection(direction);
        case <-time.After(500*time.Millisecond):
            new_order := request(order_request_channel);
            if new_order == 1 {
                floor_action := request(stop_request_channel);
                if floor_action == order.OPEN_DOOR && direction == elev.STOP {
                    open_door(door_timer, &door_open);
                } else if !door_open && direction == elev.STOP {
                    direction = request(direction_request_channel);
                    elev.SetMotorDirection(direction);
                }
            }
        }
    }
}

func open_door(door_timer *timer.Timer, door_open *bool) {
    *door_open = true;
    elev.SetMotorDirection(elev.STOP);
    door_timer.Start(3*time.Second);
    elev.SetDoorOpenLamp(true);
}

func request(request_channel chan chan int) int {
    response_channel := make(chan int);
    request_channel <-response_channel;
    value := <-response_channel;
    close(response_channel);
    return value;
}
