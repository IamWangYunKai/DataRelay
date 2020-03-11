import math
import carla
import random
import time
import numpy as np
from informer import Informer

IM_WIDTH = 480*2
IM_HEIGHT = 480
throttle = 0.0
steer = 0.0
reverse = False
brake = 0.0

class Controller(Informer):
    def parse_cmd(self, cmd):
        global throttle, steer, reverse, brake
        steer = 0.75*cmd['w']
        brake = cmd['b']
        if cmd['v'] >= 0:
            throttle = 0.7*cmd['v']
            reverse = False
        else:
            throttle = - 0.7*cmd['v']
            reverse = True
    
ifm = Controller()

def process_img(image):
    global ifm
    array = np.array(image.raw_data).reshape((image.height, image.width, 4))[:, :, :3]
    ifm.send_vision(array)

def get_norm(*args):
    _sum = 0
    for item in args:
        _sum += item*item
    return math.sqrt(_sum)

def get_sign(arg):
    return 1.0 if arg > 0 else -1.0
    
actor_list = []
try:
    client = carla.Client('localhost', 2000)
    client.set_timeout(2.0)

    world = client.get_world()

    blueprint_library = world.get_blueprint_library()
    bp = blueprint_library.filter('Tesla')[0]

    spawn_point = random.choice(world.get_map().get_spawn_points())

    vehicle = world.spawn_actor(bp, spawn_point)
    #vehicle.set_autopilot(True)  # if you just wanted some NPCs to drive.

    actor_list.append(vehicle)

    blueprint = blueprint_library.find('sensor.camera.rgb')
    # change the dimensions of the image
    blueprint.set_attribute('image_size_x', f'{IM_WIDTH}')
    blueprint.set_attribute('image_size_y', f'{IM_HEIGHT}')
    blueprint.set_attribute('fov', '120')
    # Adjust sensor relative to vehicle
    spawn_point = carla.Transform(
        location = carla.Location(x=1.5, y=0.0, z=2.0),
        rotation = carla.Rotation(pitch=-30.0, yaw=0.0, roll=0.0))
    # spawn the sensor and attach to vehicle.
    sensor = world.spawn_actor(blueprint, spawn_point, attach_to=vehicle)
    # add sensor to list of actors
    actor_list.append(sensor)
    # do something with this sensor
    sensor.listen(lambda data: process_img(data))
    
    weather = carla.WeatherParameters(
                cloudyness=random.randint(0,80),#0-100
                precipitation=random.randint(0,20),#0-100
                precipitation_deposits=random.randint(0,20),#0-100
                wind_intensity=random.randint(0,50),#0-100
                sun_azimuth_angle=random.randint(30,120),#0-360
                sun_altitude_angle=random.randint(30,90))#-90~90
        
    world.set_weather(weather)

    while True:
        vehicle.apply_control(carla.VehicleControl(
                throttle=throttle, 
                steer=steer, 
                reverse=reverse,
                manual_gear_shift=True,
                gear=1,
                brake=brake,
                hand_brake=False
                ))
        w = get_sign(vehicle.get_angular_velocity().z)*get_norm(vehicle.get_angular_velocity().x, vehicle.get_angular_velocity().y, vehicle.get_angular_velocity().z)*math.pi/180
        v = get_norm(vehicle.get_velocity().x, vehicle.get_velocity().y, vehicle.get_velocity().z)
        c = reverse
        ifm.send_cmd(v, w, c)
        time.sleep(0.016)

finally:
    print('destroying actors')
    for actor in actor_list:
        actor.destroy()
    print('done.')