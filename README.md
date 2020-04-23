# PVE Zabbix Monitoring

Утилита для съёма статистики по загрузке виртуальных машин с кластера ProxmoxPVE и отправки её на сервер Zabbix.

## Установка
    cd /opt
    git clone https://github.com/consolere/pve-zabbix-monitoring.git

## Настройка

Настройки утилиты: **pve-monitoring/pve-monitoring.conf**

Для получения данных утилите необходимо предварительно создать на ноде пользователя PVE - например monitor@pve и назначить ему прав на чтение --> Permissions ( / monitor@pve  PVEAuditor). Внести логин, пароль и URL в конфигурационный файл утилиты.

После этого можно запустить бинарный файл, если всё хорошо, он выведет текущее потребление ресурсов всеми виртуальными машинами и нодами кластера в консоль.

   
## Подключение к Zabbix

1. Проверить наличие/доустановить zabbix_sender
   
2. Добавить в конфиг заббикса /etc/zabbix/zabbix_agentd.conf
   
    ```
    ...
    ServerActive=ipaddress_zabbix-server
    ...
    UserParameter = pve.discovery,/opt/pve-monitoring/pve-monitoring -z -d
    UserParameter = pve.state,/opt/pve-monitoring/pve-monitoring -z -s
    ```
    
    Применить конфиг:
    
    ```
    systemctl restart zabbix-agent.service
    ```
    
    

3. Проверка что заббикс работает как надо:
   
        zabbix_agentd -t pve.discovery
        zabbix_agentd -t pve.state
    
4. Прикрутить в заббикс-сервере к машинке шаблон template-proxmox-pve
   
   ###### HostName в конфигурационном файле утилиты должен совпадать с именем узла сети в заббиксе !!!!!!!
