import glob
import os
import sys
import carla
import random
import time
import numpy as np
from informer import Informer

IM_WIDTH = 640
IM_HEIGHT = 480
throttle = 0.0
steer = 0.0
reverse = False

class Controller(Informer):
    def parse_cmd(self, cmd):
        global throttle, steer, reverse
        steer = cmd['w']
        if cmd['v'] >= 0:
            throttle = cmd['v']
            reverse = False
        else:
            throttle = - cmd['v']
            reverse = True
    
ifm = Controller()

def process_img(image):
    global ifm
    array = np.array(image.raw_data).reshape((image.height, image.width, 4))[:, :, :3]
    ifm.send_vision(array)

actor_list = []
try:
    client = carla.Client('localhost', 2000)
    client.set_timeout(2.0)

    world = client.get_world()

    blueprint_library = world.get_blueprint_library()

    bp = blueprint_library.filter('model3')[0]
    print(bp)

    spawn_point = random.choice(world.get_map().get_spawn_points())

    vehicle = world.spawn_actor(bp, spawn_point)
    #vehicle.apply_control(carla.VehicleControl(throttle=throttle, steer=steer, reverse=reverse))
    #vehicle.set_autopilot(True)  # if you just wanted some NPCs to drive.

    actor_list.append(vehicle)

    # https://carla.readthedocs.io/en/latest/cameras_and_sensors
    # get the blueprint for this sensor
    blueprint = blueprint_library.find('sensor.camera.rgb')
    # change the dimensions of the image
    blueprint.set_attribute('image_size_x', f'{IM_WIDTH}')
    blueprint.set_attribute('image_size_y', f'{IM_HEIGHT}')
    blueprint.set_attribute('fov', '110')

    # Adjust sensor relative to vehicle
    spawn_point = carla.Transform(carla.Location(x=2.5, z=0.7))

    # spawn the sensor and attach to vehicle.
    sensor = world.spawn_actor(blueprint, spawn_point, attach_to=vehicle)

    # add sensor to list of actors
    actor_list.append(sensor)

    # do something with this sensor
    sensor.listen(lambda data: process_img(data))

    while True:
        vehicle.apply_control(carla.VehicleControl(throttle=throttle, steer=steer, reverse=reverse))
        time.sleep(0.01)

finally:
    print('destroying actors')
    for actor in actor_list:
        actor.destroy()
    print('done.')